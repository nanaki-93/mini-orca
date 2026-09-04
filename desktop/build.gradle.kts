plugins {
  kotlin("jvm") version "2.0.21"
  kotlin("plugin.serialization") version "2.0.21"
  id("org.jetbrains.kotlin.plugin.compose") version "2.0.21"
  id("org.jetbrains.compose") version "1.7.0"
  id("io.gitlab.arturbosch.detekt") version "1.23.8"
  id("com.diffplug.spotless") version "6.25.0"
}

group = "io.miniorca"

version =
    file("../internal/version/version.go").readText().let {
      Regex("Version = \\\"([^\\\"]+)\\\"").find(it)?.groupValues?.get(1)
          ?: error("Mini-Orca version is missing")
    }

kotlin { jvmToolchain(21) }

dependencies {
  implementation(compose.desktop.currentOs)
  implementation("org.jetbrains.kotlinx:kotlinx-coroutines-swing:1.9.0")
  implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")
  testImplementation(kotlin("test"))
}

compose.desktop {
  application {
    mainClass = "io.miniorca.desktop.MainKt"
    nativeDistributions {
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
