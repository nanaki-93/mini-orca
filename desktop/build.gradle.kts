import io.gitlab.arturbosch.detekt.Detekt
import org.gradle.api.JavaVersion
import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
  kotlin("jvm") version "2.3.20"
  kotlin("plugin.serialization") version "2.3.20"
  id("org.jetbrains.kotlin.plugin.compose") version "2.3.20"
  id("org.jetbrains.compose") version "1.11.0"
  id("io.gitlab.arturbosch.detekt") version "1.23.8"
  id("com.diffplug.spotless") version "6.25.0"
}

group = "io.miniorca"

version =
    file("../internal/version/version.go").readText().let {
      Regex("Version = \\\"([^\\\"]+)\\\"").find(it)?.groupValues?.get(1)
          ?: error("Mini-Orca version is missing")
    }

kotlin {
  jvmToolchain(25)
  compilerOptions { jvmTarget.set(JvmTarget.JVM_22) }
}

java {
  sourceCompatibility = JavaVersion.VERSION_22
  targetCompatibility = JavaVersion.VERSION_22
}

dependencies {
  implementation(compose.desktop.currentOs)
  implementation("org.jetbrains.jewel:jewel-int-ui-standalone:0.40.0-262.10315.125")
  implementation("org.jetbrains.kotlinx:kotlinx-coroutines-swing:1.9.0")
  implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")
  testImplementation(kotlin("test"))
}

compose.desktop {
  application {
    mainClass = "io.miniorca.desktop.MainKt"
    nativeDistributions {
      modules("java.net.http", "jdk.unsupported")
      packageName = "Mini-Orca"
      packageVersion = project.version.toString()
    }
  }
}

tasks.test {
  useJUnitPlatform()
  providers.gradleProperty("visualOutput").orNull?.let {
    systemProperty("miniOrca.visualOutput", it)
  }
}

detekt {
  buildUponDefaultConfig = false
  config.setFrom(files("config/detekt/detekt.yml"))
  source.setFrom(files("src/main/kotlin"))
}

// Jewel requires JBR 25 at runtime, but this application does not use Java 23+ APIs.
// Keeping bytecode at 22 lets the stable Detekt 1.23.8 compiler analyze it. Upgrade
// this target when Detekt publishes stable JDK 25 support.
tasks.withType<Detekt>().configureEach { jvmTarget = "22" }

spotless {
  kotlin {
    target("src/main/kotlin/**/*.kt", "src/test/kotlin/**/*.kt")
    ktfmt("0.50")
  }
  kotlinGradle {
    target("*.gradle.kts")
    ktfmt("0.50")
  }
}

tasks.named("check") { dependsOn("detekt", "spotlessCheck") }
