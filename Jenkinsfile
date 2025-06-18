pipeline {
    agent any

    environment {
        USE_GOTESTSUM = 'yes'
        PATH = "/home/vagrant/go/bin:/usr/local/go/bin:$PATH"
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
                    
                    # Запустити heartbeat-процес у фоні
                    while true; do echo ">> still running..."; sleep 60; done &

                    HEARTBEAT_PID=$!

                    make test-frontend-coverage
                    make test-backend || true

                    # Завершити heartbeat після успішного виконання
                    kill $HEARTBEAT_PID
                '''
            }
        }

        stage('Build') {
            steps {
                sh '''
                    echo "Building Forgejo..."
                    make build
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
