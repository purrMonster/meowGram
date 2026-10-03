import 'env_loader_stub.dart'
    if (dart.library.io) 'env_loader_io.dart';

/// Cross-platform abstraction for loading environment variable overrides.
abstract class EnvLoader {
  static Future<Map<String, String>> load() => loadEnvironment();
}
