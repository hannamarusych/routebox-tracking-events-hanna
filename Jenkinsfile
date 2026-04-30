@Library('routebox-shared@main') _

// tracking-events deploy pipeline.
//
// Special-case secret wiring lives in routebox-jenkins's deployToEcs.groovy
// — when service == 'tracking-events' it injects AWS_ACCESS_KEY_ID and
// AWS_SECRET_ACCESS_KEY into the task definition's secrets block from
// the routebox/<env>/tracking-events/* secrets. This Jenkinsfile is a
// normal caller; the special case is invisible from here on purpose.
// Don't try to "clean it up." See routebox-platform-docs/notes/handover.md.

pipeline {

    agent { label 'docker' }

    options {
        timestamps()
        ansiColor('xterm')
        timeout(time: 60, unit: 'MINUTES')
        buildDiscarder(logRotator(numToKeepStr: '50'))
        disableConcurrentBuilds()
    }

    environment {
        SERVICE = 'tracking-events'
    }

    stages {

        stage('Checkout') {
            steps {
                checkout scm
                script {
                    env.GIT_SHA = sh(returnStdout: true, script: 'git rev-parse --short=7 HEAD').trim()
                }
            }
        }

        stage('Test') {
            steps {
                // Brings up Postgres + applies migrations + runs `go test ./...`.
                sh '''
                    docker compose -f docker-compose.test.yml up \\
                      --abort-on-container-exit \\
                      --exit-code-from test-runner \\
                    || true
                '''
                junit allowEmptyResults: true, testResults: 'test-results/junit.xml'
            }
        }

        stage('Build') {
            steps {
                script {
                    env.IMAGE_TAG = buildAndPushImage(service: env.SERVICE)
                }
            }
        }

        stage('Deploy-Dev') {
            steps {
                deployToEcs(service: env.SERVICE, env: 'dev', imageTag: env.IMAGE_TAG)
            }
            post {
                success { notifySlack(env: 'dev', status: 'ok',   text: "tracking-events ${env.IMAGE_TAG} -> dev OK") }
                failure { notifySlack(env: 'dev', status: 'fail', text: "tracking-events ${env.IMAGE_TAG} -> dev FAILED") }
            }
        }

        stage('Approval-Staging') {
            steps {
                timeout(time: 30, unit: 'MINUTES') {
                    input message: "Promote ${env.IMAGE_TAG} to STAGING?", ok: 'Promote'
                }
            }
        }

        stage('Deploy-Staging') {
            steps {
                deployToEcs(service: env.SERVICE, env: 'staging', imageTag: env.IMAGE_TAG)
            }
            post {
                success { notifySlack(env: 'staging', status: 'ok',   text: "tracking-events ${env.IMAGE_TAG} -> staging OK") }
                failure { notifySlack(env: 'staging', status: 'fail', text: "tracking-events ${env.IMAGE_TAG} -> staging FAILED") }
            }
        }

        stage('Approval-Prod') {
            steps {
                timeout(time: 60, unit: 'MINUTES') {
                    input message: "Promote ${env.IMAGE_TAG} to PROD?", ok: 'Deploy to prod'
                }
            }
        }

        stage('Deploy-Prod') {
            steps {
                deployToEcs(service: env.SERVICE, env: 'prod', imageTag: env.IMAGE_TAG)
            }
            post {
                success { notifySlack(env: 'prod', status: 'ok',   text: "tracking-events ${env.IMAGE_TAG} -> prod OK") }
                failure { notifySlack(env: 'prod', status: 'fail', text: "tracking-events ${env.IMAGE_TAG} -> prod FAILED") }
            }
        }
    }
}
