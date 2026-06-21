#!/bin/sh
# ----------------------------------------------------------------------------
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# ----------------------------------------------------------------------------

# ----------------------------------------------------------------------------
# Apache Maven Wrapper startup batch script, version 3.3.2
# ----------------------------------------------------------------------------

# Required env vars:
#   JAVA_HOME — location of a JDK home dir
#
# Optional env vars:
#   MAVEN_OPTS — parameters passed to the Java VM when running Maven
#   MAVEN_SKIP_RC — flag to disable loading of mavenrc files
# ----------------------------------------------------------------------------

if [ -z "$MAVEN_SKIP_RC" ]; then
  if [ -f /usr/local/etc/mavenrc ]; then
    . /usr/local/etc/mavenrc
  fi
  if [ -f /etc/mavenrc ]; then
    . /etc/mavenrc
  fi
  if [ -f "$HOME/.mavenrc" ]; then
    . "$HOME/.mavenrc"
  fi
fi

# OS specific support
cygwin=false;
darwin=false;
mingw=false
case "$(uname)" in
  CYGWIN*) cygwin=true ;;
  MINGW*) mingw=true;;
  Darwin*) darwin=true
    if [ -z "$JAVA_HOME" ] ; then
      if [ -x "/usr/libexec/java_home" ] ; then
        export JAVA_HOME="$(/usr/libexec/java_home)"
      fi
    fi
    ;;
esac

if [ -z "$JAVA_HOME" ] ; then
  if [ -r /etc/gentoo-release ] ; then
    JAVA_HOME=$(java-config --jre-home)
  fi
fi

# For Cygwin, ensure paths are in UNIX format before anything is touched
if $cygwin ; then
  [ -n "$JAVA_HOME" ] && JAVA_HOME=$(cygpath --unix "$JAVA_HOME")
  [ -n "$CLASSPATH" ] && CLASSPATH=$(cygpath --path --unix "$CLASSPATH")
fi

# For Mingw, ensure paths are in UNIX format before anything is touched
if $mingw ; then
  [ -n "$JAVA_HOME" ] && [ -d "$JAVA_HOME" ] &&
    JAVA_HOME="$(cd "$JAVA_HOME" || (echo "cannot cd to $JAVA_HOME."; exit 1); pwd)"
fi

if [ -z "$JAVA_HOME" ]; then
  javaExecutable="$(which javac)"
  if [ -n "$javaExecutable" ] && ! [ "$(expr "\"$javaExecutable\"" : '\([^ ]*\)')" = "no" ]; then
    javaHome="$(dirname "\"$(dirname \"$javaExecutable\")\"")"
    if [ -d "$javaHome" ] && [ -x "$javaHome/bin/javac" ]; then
      JAVA_HOME="$javaHome"
    fi
  fi
  if [ -z "$JAVA_HOME" ]; then
    echo "Warning: JAVA_HOME environment variable is not set." >&2
  fi
fi

# For Cygwin, switch paths to Windows format when running Maven
if $cygwin ; then
  [ -n "$JAVA_HOME" ] && JAVA_HOME=$(cygpath --path --windows "$JAVA_HOME")
fi

# Resolve the location of the Maven wrapper jar
# Use the environment variable if set, otherwise look in .mvn/wrapper/
if [ -z "$MAVEN_WRAPPER_JAR" ]; then
  MAVEN_WRAPPER_JAR="$(dirname "$(realpath "$0")")/.mvn/wrapper/maven-wrapper.jar"
fi

# Resolve the project base directory
MAVEN_PROJECTBASEDIR="${MAVEN_BASEDIR:-$(cd "$(dirname "$0")" && pwd)}"
MAVEN_PROJECTBASEDIR="$(cd "$MAVEN_PROJECTBASEDIR" && pwd)"

# Setup the classpath
CLASSPATH="$MAVEN_WRAPPER_JAR"

if [ -n "$JAVA_HOME" ]; then
  JAVACMD="$JAVA_HOME/bin/java"
else
  JAVACMD="java"
fi

# Execute Maven
exec "$JAVACMD" \
  $MAVEN_OPTS \
  -classpath "$CLASSPATH" \
  -Dmaven.multiModuleProjectDirectory="$MAVEN_PROJECTBASEDIR" \
  org.apache.maven.wrapper.MavenWrapperMain "$@"
