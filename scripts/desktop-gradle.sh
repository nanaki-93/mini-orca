#!/usr/bin/env sh

# Applies the documented launcher and toolchain locations to every desktop gate.
set -eu

script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
repo_root=$(CDPATH= cd "$script_dir/.." && pwd)
cd "$repo_root"

jdk21_home=${MINI_ORCA_JDK21_HOME:-}
java25_home=${MINI_ORCA_JAVA25_HOME:-}
java25_variable=MINI_ORCA_JAVA25_HOME
if [ -z "$java25_home" ]; then
  java25_home=${MINI_ORCA_JBR25_HOME:-}
  java25_variable=MINI_ORCA_JBR25_HOME
fi

java_major() {
  java_version_output=$("$1" -version 2>&1) || return 1
  java_version=$(printf '%s\n' "$java_version_output" | sed -n \
    -e 's/.*version "\([^"]*\)".*/\1/p' \
    -e 's/^javac \([0-9][0-9.]*\).*/\1/p' \
    -e 's/.*openjdk \([0-9][0-9.]*\).*/\1/p' | sed -n '1p')
  case "$java_version" in
    1.[0-9]*) printf '%s\n' "${java_version#1.}" | sed 's/\..*//' ;;
    [0-9]*) printf '%s' "$java_version" | sed 's/[^0-9].*$//' ;;
    *) return 1 ;;
  esac
}

is_java_home_at_major() {
  java_home=$1
  expected_major=$2
  [ -x "$java_home/bin/java" ] && [ -x "$java_home/bin/javac" ] || return 1
  java_major_value=$(java_major "$java_home/bin/java") || return 1
  javac_major_value=$(java_major "$java_home/bin/javac") || return 1
  [ "$java_major_value" = "$expected_major" ] && [ "$javac_major_value" = "$expected_major" ]
}

validate_java_home() {
  variable_name=$1
  java_home=$2
  expected_major=$3
  if [ -z "$java_home" ]; then
    return
  fi

  if [ ! -x "$java_home/bin/java" ] || [ ! -x "$java_home/bin/javac" ]; then
    printf '%s must name a JDK or JBR SDK home containing executable bin/java and bin/javac: %s\n' "$variable_name" "$java_home" >&2
    exit 2
  fi

  java_major_value=$(java_major "$java_home/bin/java") || {
    printf '%s could not determine the Java major version from %s/bin/java -version\n' "$variable_name" "$java_home" >&2
    exit 2
  }
  javac_major_value=$(java_major "$java_home/bin/javac") || {
    printf '%s could not determine the Java major version from %s/bin/javac -version\n' "$variable_name" "$java_home" >&2
    exit 2
  }
  if [ "$java_major_value" != "$expected_major" ] || [ "$javac_major_value" != "$expected_major" ]; then
    printf '%s requires Java %s; bin/java reports Java %s and bin/javac reports Java %s at %s\n' "$variable_name" "$expected_major" "$java_major_value" "$javac_major_value" "$java_home" >&2
    exit 2
  fi
}

validate_java_home MINI_ORCA_JDK21_HOME "$jdk21_home" 21
validate_java_home "$java25_variable" "$java25_home" 25

launcher_home=$jdk21_home
if [ -z "$launcher_home" ] && [ -n "${JAVA_HOME:-}" ] && is_java_home_at_major "$JAVA_HOME" 21; then
  launcher_home=$JAVA_HOME
fi
if [ -z "$launcher_home" ]; then
  path_java=$(command -v java 2>/dev/null || true)
  if [ -n "$path_java" ]; then
    path_java_home=$(CDPATH= cd "$(dirname "$path_java")/.." 2>/dev/null && pwd -P || true)
    if [ -n "$path_java_home" ] && is_java_home_at_major "$path_java_home" 21; then
      launcher_home=$path_java_home
    fi
  fi
fi
if [ -z "$launcher_home" ]; then
  printf '%s\n' 'A Java 21 JDK is required for the Gradle launcher. Set MINI_ORCA_JDK21_HOME, or set JAVA_HOME/PATH to a Java 21 JDK.' >&2
  exit 2
fi

if [ -n "$java25_home" ]; then
  exec env JAVA_HOME="$launcher_home" ./desktop/gradlew -p desktop "$@" "-Porg.gradle.java.installations.paths=$java25_home"
fi

exec env JAVA_HOME="$launcher_home" ./desktop/gradlew -p desktop "$@"
