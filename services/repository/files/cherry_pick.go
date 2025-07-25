// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"fmt"
	"strings"

	"forgejo.org/models"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/git"
	"forgejo.org/modules/log"
	"forgejo.org/modules/structs"
	"forgejo.org/services/pull"
)

// CherryPick cherrypicks or reverts a commit to the given repository
func CherryPick(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, revert bool, opts *ApplyDiffPatchOptions) (*structs.FileResponse, error) {
	if err := opts.Validate(ctx, repo, doer); err != nil {
		return nil, err
	}
	message := strings.TrimSpace(opts.Message)

	author, committer := GetAuthorAndCommitterUsers(opts.Author, opts.Committer, doer)

	t, err := NewTemporaryUploadRepository(ctx, repo)
	if err != nil {
		log.Error("NewTemporaryUploadRepository failed: %v", err)
	}
	defer t.Close()
	if err := t.Clone(opts.OldBranch, false); err != nil {
		return nil, err
	}
	if err := t.SetDefaultIndex(); err != nil {
		return nil, err
	}
	if err := t.RefreshIndex(); err != nil {
		return nil, err
	}

	// Get the commit of the original branch
	commit, err := t.GetBranchCommit(opts.OldBranch)
	if err != nil {
		return nil, err // Couldn't get a commit for the branch
	}

	// Assigned LastCommitID in opts if it hasn't been set
	if opts.LastCommitID == "" {
		opts.LastCommitID = commit.ID.String()
	} else {
		lastCommitID, err := t.gitRepo.ConvertToGitID(opts.LastCommitID)
		if err != nil {
			return nil, fmt.Errorf("CherryPick: Invalid last commit ID: %w", err)
		}
		opts.LastCommitID = lastCommitID.String()
		if commit.ID.String() != opts.LastCommitID {
			return nil, models.ErrCommitIDDoesNotMatch{
				GivenCommitID:   opts.LastCommitID,
				CurrentCommitID: opts.LastCommitID,
			}
		}
	}

	commit, err = t.GetCommit(strings.TrimSpace(opts.Content))
	if err != nil {
		return nil, err
	}
	parent, err := commit.ParentID(0)
	if err != nil {
		parent = git.ObjectFormatFromName(repo.ObjectFormatName).EmptyTree()
	}

	base, right := parent.String(), commit.ID.String()

	if revert {
		right, base = base, right
	}

	var treeHash string
	if git.SupportGitMergeTree {
		var conflict bool
		treeHash, conflict, _, err = pull.MergeTree(ctx, t.gitRepo, base, opts.LastCommitID, right, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to three-way merge %s onto %s: %w", right, opts.OldBranch, err)
		}

		if conflict {
			return nil, fmt.Errorf("failed to merge due to conflicts")
		}
	} else {
		description := fmt.Sprintf("CherryPick %s onto %s", right, opts.OldBranch)
		conflict, _, err := pull.AttemptThreeWayMerge(ctx, t.gitRepo, base, opts.LastCommitID, right, description)
		if err != nil {
			return nil, fmt.Errorf("failed to three-way merge %s onto %s: %w", right, opts.OldBranch, err)
		}

		if conflict {
			return nil, fmt.Errorf("failed to merge due to conflicts")
		}

		treeHash, err = t.WriteTree()
		if err != nil {
			// likely non-sensical tree due to merge conflicts...
			return nil, err
		}
	}

	// Now commit the tree
	var commitHash string
	if opts.Dates != nil {
		commitHash, err = t.CommitTreeWithDate("HEAD", author, committer, treeHash, message, opts.Signoff, opts.Dates.Author, opts.Dates.Committer)
	} else {
		commitHash, err = t.CommitTree("HEAD", author, committer, treeHash, message, opts.Signoff)
	}
	if err != nil {
		return nil, err
	}

	// Then push this tree to NewBranch
	if err := t.Push(doer, commitHash, opts.NewBranch); err != nil {
		return nil, err
	}

	commit, err = t.GetCommit(commitHash)
	if err != nil {
		return nil, err
	}

	fileCommitResponse, _ := GetFileCommitResponse(repo, commit) // ok if fails, then will be nil
	verification := GetPayloadCommitVerification(ctx, commit)
	fileResponse := &structs.FileResponse{
		Commit:       fileCommitResponse,
		Verification: verification,
	}

	return fileResponse, nil
}
