pipeline {
    agent { label 'aws-node' }

    environment {
        USE_GOTESTSUM = 'yes'
        PATH = "/usr/local/go/bin:$PATH"
        AWS_REGION = 'us-east-1'
        AWS_ACCOUNT_ID = '635670595114'
        ECR_REPO_NAME = 'forgejo/app'
        DISCORD_WEBHOOK = credentials('DISCORD_WEBHOOK')

        DB_HOST = credentials('DB_HOST')
        DB_PORT = 3306
        DB_USER = credentials('DB_USER')
        DB_PASS = credentials('DB_PASS')

        FORGEJO_DOMAIN = 'forgejo.pp.ua'
        FORGEJO_PORT = 3000
        FORGEJO_SSH_PORT = 22
                
        FORGEJO_LFS_JWT_SECRET = credentials('FORGEJO_LFS_JWT_SECRET')
        FORGEJO_INTERNAL_TOKEN = credentials('FORGEJO_INTERNAL_TOKEN')
        FORGEJO_JWT_SECRET = credentials('FORGEJO_JWT_SECRET')
    }

    stages {
        stage('SonarQube analysis') {
            steps {
                script {
                    scannerHome = tool '<SonarScanner>'
                }
                withSonarQubeEnv('SonarQube Cloud') {
                    sh "${scannerHome}/bin/sonar-scanner"
                }
            }
        }
      
        stage('Dependencies') {
            steps {
                sh '''
                    echo "Downloading Go dependencies..."
                    go mod tidy
                '''
            }
        }

        // stage('Static Checks') {
        //     parallel {
                // stage('Lint') {
                //     steps {
                //         sh '''
                //             echo "Running linter..."
                //             golangci-lint run --timeout 15m --verbose
                //         '''
                //     }
                // }

                

        //         stage('SonarQube Analysis') {
        //             steps {
        //                 script {
        //                     def scannerHome = tool 'sonarscanner'
        //                     withSonarQubeEnv() {
        //                         sh """
        //                             ${scannerHome}/bin/sonar-scanner \
        //                                 -Dsonar.projectKey=forgejo \
        //                                 -Dsonar.sources=. \
        //                                 -Dsonar.go.coverage.reportPaths=coverage.out
        //                         """
        //                     }
        //                 }
        //             }
        //         }
        //     }
        // }

        stage('Tests') {
            steps {
                sh '''
                    echo "Running tests..."

                    make test-frontend-coverage
                    make test-backend || true
                '''
            }
        }

      
        stage('Build') {
            steps {
                sh '''
                    echo "Building Forgejo..."
                    CGO_ENABLED=0 TAGS="bindata" make build
                '''
            }
        }

        stage('Init Tag Info') {
            steps {
                script {
                    def branch = env.GIT_BRANCH?.replaceAll(/^origin\//, '')?.replaceAll('/', '-') ?: 'unknown'
                    def shortCommit = env.GIT_COMMIT?.take(7) ?: '0000000'
                    def timestamp = new Date(currentBuild.startTimeInMillis).format("yyyyMMdd-HHmm", TimeZone.getTimeZone('UTC'))
                    env.IMAGE_TAG = "${branch}-${shortCommit}-${timestamp}"
                }
            }
        }

        stage('Docker Build & Push to ECR') {
            environment {
                AWS_ACCESS_KEY_ID = credentials('aws-access-key-id')
                AWS_SECRET_ACCESS_KEY = credentials('aws-secret-access-key')
            }
            steps {
                sh '''
                    echo "Logging into ECR..."
                    aws ecr get-login-password --region $AWS_REGION | \
                        docker login --username AWS --password-stdin $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com

                    echo "Building Docker image..."
                    docker build -t forgejo-app .

                    echo "Tagging image for ECR..."
                    docker tag forgejo-app:latest $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/$ECR_REPO_NAME:$IMAGE_TAG
                    docker tag forgejo-app:latest $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/$ECR_REPO_NAME:latest

                    echo "Pushing image to ECR..."
                    docker push $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/$ECR_REPO_NAME:$IMAGE_TAG
                    docker push $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/$ECR_REPO_NAME:latest
                '''
            }
        }
    }

    post {
        success {
            discordSend(
              webhookURL: env.DISCORD_WEBHOOK,
              title: env.JOB_NAME,
              description: "SUCCESS: build #${env.BUILD_NUMBER}",
              link: env.BUILD_URL,
              result: 'SUCCESS'
            )
        }
        failure {
            discordSend(
              webhookURL: env.DISCORD_WEBHOOK,
              title: env.JOB_NAME,
              description: "FAILED: build #${env.BUILD_NUMBER}",
              link: env.BUILD_URL,
              result: 'FAILURE'
            )
        }
    }
}
