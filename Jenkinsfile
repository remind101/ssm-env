#!/usr/bin/env groovy

pipeline {

  agent any

  environment {
    AWS_REGION = 'ap-southeast-2'
    APP_NAME = 'ssm-env'
  }

  stages {
    stage('build') {
      steps {
        sh 'docker-compose run build'
      }
    }

    stage('test') {
      steps {
        sh 'docker-compose run test'
      }
    }


    stage('release') {
      when {
        branch 'intellihr'
      }
      steps {
        withAWSParameterStore(namePrefixes: '/jenkins/GITHUB_USERNAME,/jenkins/GITHUB_API_TOKEN', regionName: env.AWS_REGION) {
          sh 'docker-compose run --rm build cp bin/ssm-env /output'
          sh '.ci/scripts/release.sh'
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
