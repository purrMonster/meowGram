import 'dart:io';
import 'package:flutter/foundation.dart';

/// Native/desktop implementation reading OS environment and local .env files via dart:io.
Future<Map<String, String>> loadEnvironment() async {
  final result = <String, String>{};

  // 1. Inherit from OS process environment variables
  try {
    result.addAll(Platform.environment);
  } catch (e) {
    debugPrint('AppConfig notice reading process environment: $e');
  }

  // 2. Candidate .env file locations (checked in precedence order)
  final candidatePaths = <String>[
    '.env',
    'client/.env',
    '../.env',
    '../deploy/.env',
    'deploy/.env',
    '../../deploy/.env',
  ];

  for (final path in candidatePaths) {
    try {
      final file = File(path);
      if (await file.exists()) {
        final lines = await file.readAsLines();
        for (var line in lines) {
          line = line.trim();
          if (line.isEmpty || line.startsWith('#')) continue;
          final eqIdx = line.indexOf('=');
          if (eqIdx <= 0) continue;
          final key = line.substring(0, eqIdx).trim();
          var val = line.substring(eqIdx + 1).trim();

          // Strip surrounding single or double quotes
          if ((val.startsWith('"') && val.endsWith('"')) ||
              (val.startsWith("'") && val.endsWith("'"))) {
            if (val.length >= 2) {
              val = val.substring(1, val.length - 1);
            }
          }
          if (key.isNotEmpty) {
            result[key] = val;
          }
        }
        debugPrint('AppConfig: Successfully loaded .env configuration from $path');
        break; // Stop after first resolved file
      }
    } catch (e) {
      debugPrint('AppConfig notice checking $path: $e');
    }
  }

  return result;
}
