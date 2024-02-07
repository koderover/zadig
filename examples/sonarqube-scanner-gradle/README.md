# SonarQube Scanner for Gradle

This example demonstrates how to analyze a simple project built with [Gradle](https://gradle.org/).

## Prerequisites

* [SonarQube](http://www.sonarqube.org/downloads/) 9.9 LTS or Latest
* A gradle wrapper is included that bundles gradle. All other required plugins will be pulled by gradle as needed.

## Usage

Run the following command (updating the sonar.host.url property as appropriate):

* On Unix-like systems:
  `./gradlew -Dsonar.host.url=http://myhost:9000 sonar`
* On Windows:
  `.\gradlew.bat -D'sonar.host.url=http://myhost:9000' sonar`

## Coverage

To get the project [test coverage](https://community.sonarsource.com/t/coverage-test-data-importing-jacoco-coverage-report-in-xml-format) computed, add gradle task `jacocoTestReport` to your command line.
