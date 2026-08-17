plugins {
    kotlin("jvm") version "2.0.21"
    kotlin("plugin.serialization") version "2.0.21"
    id("org.jetbrains.kotlin.plugin.compose") version "2.0.21"
    id("org.jetbrains.compose") version "1.7.0"
}

group = "io.miniorca"
version = "4.1.0"

kotlin {
    jvmToolchain(21)
}

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
            packageVersion = "4.1.0"
        }
    }
}

tasks.test {
    useJUnitPlatform()
}
