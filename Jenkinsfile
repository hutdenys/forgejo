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

        stage('Lint') {
            steps {
                sh '''
                    echo "Linting..."
                    golangci-lint run --timeout 5m
                '''
            }
        }

        stage('Unit Tests') {
            steps {
                sh '''
                    echo "Running unit tests..."
                    make test
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
            echo 'Pipeline ends succesfully'
        }
        failure {
            echo 'Pipeline ends with errors.'
        }
    }
}
