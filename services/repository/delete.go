// Copyright 2023 The Gitea Authors. All rights reserved.
// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	"forgejo.org/models"
	actions_model "forgejo.org/models/actions"
	activities_model "forgejo.org/models/activities"
	admin_model "forgejo.org/models/admin"
	asymkey_model "forgejo.org/models/asymkey"
	"forgejo.org/models/db"
	git_model "forgejo.org/models/git"
	issues_model "forgejo.org/models/issues"
	"forgejo.org/models/organization"
	packages_model "forgejo.org/models/packages"
	access_model "forgejo.org/models/perm/access"
	project_model "forgejo.org/models/project"
	repo_model "forgejo.org/models/repo"
	secret_model "forgejo.org/models/secret"
	system_model "forgejo.org/models/system"
	user_model "forgejo.org/models/user"
	"forgejo.org/models/webhook"
	actions_module "forgejo.org/modules/actions"
	"forgejo.org/modules/lfs"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/storage"
	federation_service "forgejo.org/services/federation"

	"xorm.io/builder"
)

// DeleteRepository deletes a repository for a user or organization.
// make sure if you call this func to close open sessions (sqlite will otherwise get a deadlock)
func DeleteRepositoryDirectly(ctx context.Context, doer *user_model.User, repoID int64, ignoreOrgTeams ...bool) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	repo := &repo_model.Repository{}
	has, err := sess.ID(repoID).Get(repo)
	if err != nil {
		return err
	} else if !has {
		return repo_model.ErrRepoNotExist{
			ID:        repoID,
			OwnerName: "",
			Name:      "",
		}
	}

	// Query the action tasks of this repo, they will be needed after they have been deleted to remove the logs
	tasks, err := db.Find[actions_model.ActionTask](ctx, actions_model.FindTaskOptions{RepoID: repoID})
	if err != nil {
		return fmt.Errorf("find actions tasks of repo %v: %w", repoID, err)
	}

	// Query the artifacts of this repo, they will be needed after they have been deleted to remove artifacts files in ObjectStorage
	artifacts, err := db.Find[actions_model.ActionArtifact](ctx, actions_model.FindArtifactsOptions{RepoID: repoID})
	if err != nil {
		return fmt.Errorf("list actions artifacts of repo %v: %w", repoID, err)
	}

	// In case owner is a organization, we have to change repo specific teams
	// if ignoreOrgTeams is not true
	var org *user_model.User
	if len(ignoreOrgTeams) == 0 || !ignoreOrgTeams[0] {
		if org, err = user_model.GetUserByID(ctx, repo.OwnerID); err != nil {
			return err
		}
	}

	// Delete Deploy Keys
	deployKeys, err := db.Find[asymkey_model.DeployKey](ctx, asymkey_model.ListDeployKeysOptions{RepoID: repoID})
	if err != nil {
		return fmt.Errorf("listDeployKeys: %w", err)
	}
	needRewriteKeysFile := len(deployKeys) > 0
	for _, dKey := range deployKeys {
		if err := models.DeleteDeployKey(ctx, doer, dKey.ID); err != nil {
			return fmt.Errorf("deleteDeployKeys: %w", err)
		}
	}

	// If the repository was reported as abusive, a shadow copy should be created before deletion.
	if err := repo_model.IfNeededCreateShadowCopyForRepository(ctx, repo, false); err != nil {
		return err
	}

	if cnt, err := sess.ID(repoID).Delete(&repo_model.Repository{}); err != nil {
		return err
	} else if cnt != 1 {
		return repo_model.ErrRepoNotExist{
			ID:        repoID,
			OwnerName: "",
			Name:      "",
		}
	}

	if org != nil && org.IsOrganization() {
		teams, err := organization.FindOrgTeams(ctx, org.ID)
		if err != nil {
			return err
		}
		for _, t := range teams {
			if !organization.HasTeamRepo(ctx, t.OrgID, t.ID, repoID) {
				continue
			} else if err = removeRepositoryFromTeam(ctx, t, repo, false); err != nil {
				return err
			}
		}
	}

	attachments := make([]*repo_model.Attachment, 0, 20)
	if err = sess.Join("INNER", "`release`", "`release`.id = `attachment`.release_id").
		Where("`release`.repo_id = ?", repoID).
		Find(&attachments); err != nil {
		return err
	}
	releaseAttachments := make([]string, 0, len(attachments))
	for i := 0; i < len(attachments); i++ {
		releaseAttachments = append(releaseAttachments, attachments[i].RelativePath())
	}

	if _, err := db.Exec(ctx, "UPDATE `user` SET num_stars=num_stars-1 WHERE id IN (SELECT `uid` FROM `star` WHERE repo_id = ?)", repo.ID); err != nil {
		return err
	}

	if setting.Database.Type.IsMySQL() {
		// mariadb:10 does not use the hook_task KEY when using IN.
		// https://codeberg.org/forgejo/forgejo/issues/3678
		//
		// Version 11 does support it, but is not available in debian yet.
		// Version 11.4 LTS is not available yet (stable should be released mid 2024 https://mariadb.org/mariadb/all-releases/)

		// Sqlite does not support the DELETE *** FROM *** syntax
		// https://stackoverflow.com/q/24511153/3207406

		// in the meantime, use a dedicated query for mysql...
		if _, err := db.Exec(ctx, "DELETE `hook_task` FROM `hook_task` INNER JOIN `webhook` ON `webhook`.id = `hook_task`.hook_id WHERE `webhook`.repo_id = ?", repo.ID); err != nil {
			return err
		}
	} else {
		if _, err := db.GetEngine(ctx).In("hook_id", builder.Select("id").From("webhook").Where(builder.Eq{"webhook.repo_id": repo.ID})).
			Delete(&webhook.HookTask{}); err != nil {
			return err
		}
	}

	if err := db.DeleteBeans(ctx,
		&access_model.Access{RepoID: repo.ID},
		&activities_model.Action{RepoID: repo.ID},
		&repo_model.Collaboration{RepoID: repoID},
		&issues_model.Comment{RefRepoID: repoID},
		&git_model.CommitStatus{RepoID: repoID},
		&git_model.Branch{RepoID: repoID},
		&git_model.LFSLock{RepoID: repoID},
		&repo_model.LanguageStat{RepoID: repoID},
		&issues_model.Milestone{RepoID: repoID},
		&repo_model.Mirror{RepoID: repoID},
		&activities_model.Notification{RepoID: repoID},
		&git_model.ProtectedBranch{RepoID: repoID},
		&git_model.ProtectedTag{RepoID: repoID},
		&repo_model.PushMirror{RepoID: repoID},
		&repo_model.Release{RepoID: repoID},
		&repo_model.RepoIndexerStatus{RepoID: repoID},
		&repo_model.Redirect{RedirectRepoID: repoID},
		&repo_model.RepoUnit{RepoID: repoID},
		&repo_model.Star{RepoID: repoID},
		&admin_model.Task{RepoID: repoID},
		&repo_model.Watch{RepoID: repoID},
		&webhook.Webhook{RepoID: repoID},
		&secret_model.Secret{RepoID: repoID},
		&actions_model.ActionTaskStep{RepoID: repoID},
		&actions_model.ActionTask{RepoID: repoID},
		&actions_model.ActionRunJob{RepoID: repoID},
		&actions_model.ActionRun{RepoID: repoID},
		&actions_model.ActionRunner{RepoID: repoID},
		&actions_model.ActionScheduleSpec{RepoID: repoID},
		&actions_model.ActionSchedule{RepoID: repoID},
		&actions_model.ActionArtifact{RepoID: repoID},
		&repo_model.RepoArchiveDownloadCount{RepoID: repoID},
		&actions_model.ActionRunnerToken{RepoID: repoID},
	); err != nil {
		return fmt.Errorf("deleteBeans: %w", err)
	}

	// Delete Labels and related objects
	if err := issues_model.DeleteLabelsByRepoID(ctx, repoID); err != nil {
		return err
	}

	// Delete Pulls and related objects
	if err := issues_model.DeletePullsByBaseRepoID(ctx, repoID); err != nil {
		return err
	}

	// Delete Issues and related objects
	var attachmentPaths []string
	if attachmentPaths, err = issues_model.DeleteIssuesByRepoID(ctx, repoID); err != nil {
		return err
	}

	// Delete issue index
	if err := db.DeleteResourceIndex(ctx, "issue_index", repoID); err != nil {
		return err
	}

	if repo.IsFork {
		if _, err := db.Exec(ctx, "UPDATE `repository` SET num_forks=num_forks-1 WHERE id=?", repo.ForkID); err != nil {
			return fmt.Errorf("decrease fork count: %w", err)
		}
	}

	if _, err := db.Exec(ctx, "UPDATE `user` SET num_repos=num_repos-1 WHERE id=?", repo.OwnerID); err != nil {
		return err
	}

	if len(repo.Topics) > 0 {
		if err := repo_model.RemoveTopicsFromRepo(ctx, repo.ID); err != nil {
			return err
		}
	}

	if err := project_model.DeleteProjectByRepoID(ctx, repoID); err != nil {
		return fmt.Errorf("unable to delete projects for repo[%d]: %w", repoID, err)
	}

	// Remove LFS objects
	var lfsObjects []*git_model.LFSMetaObject
	if err = sess.Where("repository_id=?", repoID).Find(&lfsObjects); err != nil {
		return err
	}

	lfsPaths := make([]string, 0, len(lfsObjects))
	for _, v := range lfsObjects {
		count, err := db.CountByBean(ctx, &git_model.LFSMetaObject{Pointer: lfs.Pointer{Oid: v.Oid}})
		if err != nil {
			return err
		}
		if count > 1 {
			continue
		}

		lfsPaths = append(lfsPaths, v.RelativePath())
	}

	if _, err := db.DeleteByBean(ctx, &git_model.LFSMetaObject{RepositoryID: repoID}); err != nil {
		return err
	}

	// Remove archives
	var archives []*repo_model.RepoArchiver
	if err = sess.Where("repo_id=?", repoID).Find(&archives); err != nil {
		return err
	}

	archivePaths := make([]string, 0, len(archives))
	for _, v := range archives {
		archivePaths = append(archivePaths, v.RelativePath())
	}

	if _, err := db.DeleteByBean(ctx, &repo_model.RepoArchiver{RepoID: repoID}); err != nil {
		return err
	}

	if repo.NumForks > 0 {
		if _, err = sess.Exec("UPDATE `repository` SET fork_id=0,is_fork=? WHERE fork_id=?", false, repo.ID); err != nil {
			log.Error("reset 'fork_id' and 'is_fork': %v", err)
		}
	}

	// Get all attachments with both issue_id and release_id are zero
	var newAttachments []*repo_model.Attachment
	if err := sess.Where(builder.Eq{
		"repo_id":    repo.ID,
		"issue_id":   0,
		"release_id": 0,
	}).Find(&newAttachments); err != nil {
		return err
	}

	newAttachmentPaths := make([]string, 0, len(newAttachments))
	for _, attach := range newAttachments {
		newAttachmentPaths = append(newAttachmentPaths, attach.RelativePath())
	}

	if _, err := sess.Where("repo_id=?", repo.ID).Delete(new(repo_model.Attachment)); err != nil {
		return err
	}

	if err := federation_service.DeleteFollowingRepos(ctx, repo.ID); err != nil {
		return err
	}

	// unlink packages linked to this repository
	if err = packages_model.UnlinkRepositoryFromAllPackages(ctx, repoID); err != nil {
		return err
	}

	if err = committer.Commit(); err != nil {
		return err
	}

	committer.Close()

	if needRewriteKeysFile {
		if err := asymkey_model.RewriteAllPublicKeys(ctx); err != nil {
			log.Error("RewriteAllPublicKeys failed: %v", err)
		}
	}

	// We should always delete the files after the database transaction succeed. If
	// we delete the file but the database rollback, the repository will be broken.

	// Remove repository files.
	repoPath := repo.RepoPath()
	system_model.RemoveAllWithNotice(ctx, "Delete repository files", repoPath)

	// Remove wiki files
	if repo.HasWiki() {
		system_model.RemoveAllWithNotice(ctx, "Delete repository wiki", repo.WikiPath())
	}

	// Remove archives
	for _, archive := range archivePaths {
		system_model.RemoveStorageWithNotice(ctx, storage.RepoArchives, "Delete repo archive file", archive)
	}

	// Remove lfs objects
	for _, lfsObj := range lfsPaths {
		system_model.RemoveStorageWithNotice(ctx, storage.LFS, "Delete orphaned LFS file", lfsObj)
	}

	// Remove issue attachment files.
	for _, attachment := range attachmentPaths {
		system_model.RemoveStorageWithNotice(ctx, storage.Attachments, "Delete issue attachment", attachment)
	}

	// Remove release attachment files.
	for _, releaseAttachment := range releaseAttachments {
		system_model.RemoveStorageWithNotice(ctx, storage.Attachments, "Delete release attachment", releaseAttachment)
	}

	// Remove attachment with no issue_id and release_id.
	for _, newAttachment := range newAttachmentPaths {
		system_model.RemoveStorageWithNotice(ctx, storage.Attachments, "Delete issue attachment", newAttachment)
	}

	if len(repo.Avatar) > 0 {
		if err := storage.RepoAvatars.Delete(repo.CustomAvatarRelativePath()); err != nil {
			return fmt.Errorf("Failed to remove %s: %w", repo.Avatar, err)
		}
	}

	// Finally, delete action logs after the actions have already been deleted to avoid new log files
	for _, task := range tasks {
		err := actions_module.RemoveLogs(ctx, task.LogInStorage, task.LogFilename)
		if err != nil {
			log.Error("remove log file %q: %v", task.LogFilename, err)
			// go on
		}
	}

	// delete actions artifacts in ObjectStorage after the repo have already been deleted
	for _, art := range artifacts {
		if err := storage.ActionsArtifacts.Delete(art.StoragePath); err != nil {
			log.Error("remove artifact file %q: %v", art.StoragePath, err)
			// go on
		}
	}

	return nil
}

// removeRepositoryFromTeam removes a repository from a team and recalculates access
// Note: Repository shall not be removed from team if it includes all repositories (unless the repository is deleted)
func removeRepositoryFromTeam(ctx context.Context, t *organization.Team, repo *repo_model.Repository, recalculate bool) (err error) {
	e := db.GetEngine(ctx)
	if err = organization.RemoveTeamRepo(ctx, t.ID, repo.ID); err != nil {
		return err
	}

	t.NumRepos--
	if _, err = e.ID(t.ID).Cols("num_repos").Update(t); err != nil {
		return err
	}

	// Don't need to recalculate when delete a repository from organization.
	if recalculate {
		if err = access_model.RecalculateTeamAccesses(ctx, repo, t.ID); err != nil {
			return err
		}
	}

	teamUsers, err := organization.GetTeamUsersByTeamID(ctx, t.ID)
	if err != nil {
		return fmt.Errorf("getTeamUsersByTeamID: %w", err)
	}
	for _, teamUser := range teamUsers {
		has, err := access_model.HasAccess(ctx, teamUser.UID, repo)
		if err != nil {
			return err
		} else if has {
			continue
		}

		if err = repo_model.WatchRepo(ctx, teamUser.UID, repo.ID, false); err != nil {
			return err
		}

		// Remove all IssueWatches a user has subscribed to in the repositories
		if err := issues_model.RemoveIssueWatchersByRepoID(ctx, teamUser.UID, repo.ID); err != nil {
			return err
		}
	}

	return nil
}

// HasRepository returns true if given repository belong to team.
func HasRepository(ctx context.Context, t *organization.Team, repoID int64) bool {
	return organization.HasTeamRepo(ctx, t.OrgID, t.ID, repoID)
}

// RemoveRepositoryFromTeam removes repository from team of organization.
// If the team shall include all repositories the request is ignored.
func RemoveRepositoryFromTeam(ctx context.Context, t *organization.Team, repoID int64) error {
	if !HasRepository(ctx, t, repoID) {
		return nil
	}

	if t.IncludesAllRepositories {
		return nil
	}

	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return err
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = removeRepositoryFromTeam(ctx, t, repo, true); err != nil {
		return err
	}

	return committer.Commit()
}

// DeleteOwnerRepositoriesDirectly calls DeleteRepositoryDirectly for all repos of the given owner
func DeleteOwnerRepositoriesDirectly(ctx context.Context, owner *user_model.User) error {
	for {
		repos, _, err := repo_model.GetUserRepositories(ctx, &repo_model.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: repo_model.RepositoryListDefaultPageSize,
				Page:     1,
			},
			Private: true,
			OwnerID: owner.ID,
			Actor:   owner,
		})
		if err != nil {
			return fmt.Errorf("GetUserRepositories: %w", err)
		}
		if len(repos) == 0 {
			break
		}
		for _, repo := range repos {
			if err := DeleteRepositoryDirectly(ctx, owner, repo.ID); err != nil {
				return fmt.Errorf("unable to delete repository %s for %s[%d]. Error: %w", repo.Name, owner.Name, owner.ID, err)
			}
		}
	}
	return nil
}
