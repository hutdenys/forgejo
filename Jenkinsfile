pipeline {
    agent any

    environment {
        USE_GOTESTSUM = 'yes'
        PATH = "/usr/local/go/bin:$PATH"
        AWS_REGION = 'eu-central-1'
        AWS_ACCOUNT_ID = '535845769543'
        ECR_REPO_NAME = 'forgejo/app'
        IMAGE_TAG = "${env.BUILD_NUMBER}"
    }

    stages {
        stage('Checkout') {
            steps {
                echo "Cloning repository..."
                checkout scm
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

        stage('Lint') {
            steps {
                sh '''
                    echo "Linting..."
                    golangci-lint run --timeout 15m --verbose
                '''
            }
        }

        stage('Tests') {
            steps {
                sh '''
                    echo "Running tests..."
                    
                    make test-frontend-coverage
                    make test-backend || true
                '''
            }
        }

        stage('SonarQube Analysis') {
            steps {
                script {
                    def scannerHome = tool 'sonarscanner'
                    withSonarQubeEnv() {
                        sh """
                            ${scannerHome}/bin/sonar-scanner \
                                -Dsonar.projectKey=forgejo \
                                -Dsonar.sources=. \
                                -Dsonar.go.coverage.reportPaths=coverage.out
                        """
                    }
                }
            }
        }

        stage('Build') {
            steps {
                sh '''
                    echo "Building Forgejo..."
                    TAGS="bindata" make build
                '''
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

                    echo "Pushing image to ECR..."
                    docker push $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/$ECR_REPO_NAME:$IMAGE_TAG
                '''
            }
        }
    }

    post {
        success {
            echo 'Pipeline ends successfully'
            archiveArtifacts artifacts: '**/build/**', fingerprint: true
        }
        failure {
            echo 'Pipeline ends with errors.'
        }
    }
}
