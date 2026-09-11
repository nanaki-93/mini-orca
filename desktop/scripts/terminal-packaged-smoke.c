/* Validation-only launcher: initialize the actual jlink runtime without a java binary. */
#include <dlfcn.h>
#include <jni.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int main(int argc, char **argv) {
  if (argc != 3) {
    fputs("usage: terminal-packaged-smoke <bundled-libjvm> <classpath>\n", stderr);
    return 2;
  }
  void *library = dlopen(argv[1], RTLD_NOW | RTLD_GLOBAL);
  if (!library) {
    fprintf(stderr, "Cannot load bundled JVM: %s\n", dlerror());
    return 2;
  }
  typedef jint(JNICALL *CreateVM)(JavaVM **, void **, void *);
  CreateVM create_vm = (CreateVM)dlsym(library, "JNI_CreateJavaVM");
  if (!create_vm) {
    fputs("Bundled JVM has no JNI_CreateJavaVM\n", stderr);
    return 2;
  }
  const char *prefix = "-Djava.class.path=";
  char *classpath = malloc(strlen(prefix) + strlen(argv[2]) + 1);
  if (!classpath) return 2;
  strcpy(classpath, prefix);
  strcat(classpath, argv[2]);
  JavaVMOption options[] = {
      {classpath, NULL}, {"--enable-native-access=ALL-UNNAMED", NULL}};
  JavaVMInitArgs args = {JNI_VERSION_1_8, 2, options, JNI_FALSE};
  JavaVM *vm = NULL;
  JNIEnv *env = NULL;
  jint result = create_vm(&vm, (void **)&env, &args);
  free(classpath);
  if (result != JNI_OK) {
    fprintf(stderr, "Bundled JVM initialization failed: %d\n", result);
    return 2;
  }
  jclass probe = (*env)->FindClass(env, "io/miniorca/desktop/DesktopTerminalSessionTestKt");
  if (probe) {
    jmethodID run = (*env)->GetStaticMethodID(env, probe, "main", "()V");
    if (run) (*env)->CallStaticVoidMethod(env, probe, run);
  }
  int failed = (*env)->ExceptionCheck(env);
  if (failed) {
    (*env)->ExceptionDescribe(env);
    (*env)->ExceptionClear(env);
  }
  if ((*vm)->DestroyJavaVM(vm) != JNI_OK) failed = 1;
  return failed ? 1 : 0;
}
