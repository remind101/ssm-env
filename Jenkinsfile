#!/usr/bin/env groovy

pipeline {

  agent any

  environment {
    AWS_REGION = 'ap-southeast-2'
    APP_NAME = 'ssm-env'
  }

  stages {
    stage('prepare') {
      steps {
        script {
          env.COMMIT_HASH = sh(returnStdout: true, script: 'git rev-parse HEAD').trim().take(7)
        }
      }
    }

    stage('build') {
      steps {
        sh 'docker-compose build build test'
      }
    }

    stage('test') {
      steps {
        sh 'docker-compose run --rm test'
      }
    }

    stage('release') {
      when {
        branch 'intellihr'
      }
      steps {
        withAWSParameterStore(namePrefixes: '/jenkins/GITHUB_USERNAME,/jenkins/GITHUB_API_TOKEN', regionName: env.AWS_REGION) {
          sh 'docker-compose run --rm build /output/.ci/scripts/release.sh'
        }
      }
    }
  }

  post {
    success {
      slackSend color: 'good',
                message: "[*${env.APP_NAME}*:${env.BRANCH_NAME}] (<https://github.com/intellihr/${env.APP_NAME}/commit/${env.COMMIT_HASH}|${env.COMMIT_HASH}>) *SUCCESSFUL* <${env.BUILD_URL}|#${env.BUILD_NUMBER}>"
    }
    failure {
      slackSend color: 'danger',
                message: "[*${env.APP_NAME}*:${env.BRANCH_NAME}] (<https://github.com/intellihr/${env.APP_NAME}/commit/${env.COMMIT_HASH}|${env.COMMIT_HASH}>) *FAILED* <${env.BUILD_URL}|#${env.BUILD_NUMBER}>"
    }
    always {
      sh 'docker-compose rm -f -s -v || true'
      sh 'docker run --rm -v $(pwd):/tmp alpine chown -R $(id -u) /tmp'
      cleanWs()
    }
  }
}
