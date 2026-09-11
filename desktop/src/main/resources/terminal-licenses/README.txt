Mini-Orca terminal dependencies

JediTerm core/ui 3.72, JetBrains and contributors. Mini-Orca selects Apache-2.0
from the upstream dual Apache-2.0/LGPL-3.0 license. The published version's
README explicitly permits that choice even though its Maven POM lists LGPL only.
Source revision: 97c49c88c4a41c2213072281f5c936ecb8753fb5 (VERSION = 3.72).
https://github.com/JetBrains/jediterm/tree/97c49c88c4a41c2213072281f5c936ecb8753fb5
License: jediterm-APACHE.txt (upstream license wording).

Pty4J 0.13.12, JetBrains and contributors, Eclipse Public License 1.0.
License: pty4j-LICENSE.txt. Upstream attribution: pty4j-NOTICE.txt.
Unmodified corresponding sources:
https://repo.maven.apache.org/maven2/org/jetbrains/pty4j/pty4j/0.13.12/pty4j-0.13.12-sources.jar
Project: https://github.com/JetBrains/pty4j
The dependency jar also contains native binaries for other hosts. WinPty and
Microsoft OpenConsole/ConPTY use the MIT notices supplied here; their presence
is not a Mini-Orca support claim for Windows.
https://github.com/rprichard/winpty/blob/0.4.3/LICENSE
https://github.com/microsoft/terminal/blob/main/LICENSE

JNA and JNA Platform 5.17.0 (already used by Jewel), Timothy Wall and contributors.
Mini-Orca selects Apache-2.0; the included jediterm-APACHE.txt supplies that full
license text. jna-5.17.0-LICENSE.txt is the license declaration from the JNA jar.
JNA's libffi notice is retained as jna-libffi-LICENSE.txt.
https://github.com/java-native-access/jna/tree/5.17.0

SLF4J API 2.0.13, QOS.ch Sarl and contributors, MIT.
slf4j-api-2.0.13-LICENSE.txt is copied from its META-INF/LICENSE.txt.
https://www.slf4j.org/license.html

Only line endings and trailing whitespace are normalized; license wording is preserved.
These files are packaged as application resources in terminal-licenses/.
The dependency jars remain separate and unmodified. Mini-Orca terminal support
is currently verified only for the locally built macOS arm64 distribution.
