pipeline {
    agent any

    environment {
        WH_DOCKER_AUTH = credentials('WH_DOCKER_AUTH')
        WH_DOCKER_PUSH_PREFIX = credentials('WH_DOCKER_PUSH_PREFIX')
        WH_S3_ACCESS_KEY_ID = credentials('WH_S3_ACCESS_KEY_ID')
        WH_S3_SECRET_ACCESS_KEY = credentials('WH_S3_SECRET_ACCESS_KEY')
        WH_S3_URL = credentials('WH_S3_URL')
        NPM_CONFIG_UNSAFE_PERM = 'true'
    }

    stages {
        stage('Install') {
            steps {
                ansiColor('xterm') {
                    sh 'npm install'
                    sh 'npx wheelhouse install'
                }
            }
        }
        stage('Build') {
            steps {
                ansiColor('xterm') {
                    sh 'npx wheelhouse build'
                }
            }
        }
        stage('Push') {
            steps {
                ansiColor('xterm') {
                    sh 'npx wheelhouse push'
                }
            }
        }
    }

}
