pipeline {
    agent any

    environment {
        USE_GOTESTSUM = 'yes'
        PATH = "/usr/local/go/bin:$PATH"
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

        stage('Build') {
            steps {
                sh '''
                    echo "Building Forgejo..."
                    TAGS="bindata" make build
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
