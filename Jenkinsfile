pipeline {
    agent any

    environment {
        PROJECT_NAME = 'forgejo-devops'
    }

    stages {
        stage('Checkout') {
            steps {
                echo "Cloning repository..."
                checkout scm
            }
        }

        stage('Build') {
            steps {
                echo "Build stage (placeholder) — add your build commands here"
                // Наприклад: sh 'make build'
            }
        }

        stage('Test') {
            steps {
                echo "Test stage (placeholder) — add your tests here"
                // Наприклад: sh 'make test'
            }
        }

        stage('Deploy') {
            steps {
                echo "Deploy stage (placeholder) — insert Ansible or other deploy logic"
                // Наприклад:
                // sh 'ansible-playbook -i inventory playbook.yml'.
            }
        }
    }

    post {
        success {
            echo "Pipeline for ${env.PROJECT_NAME} completed successfully!"
        }
        failure {
            echo "Pipeline for ${env.PROJECT_NAME} failed."
        }
    }
}
