import 'dart:io';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('ios/Runner/Info.plist contains meowgram custom URL scheme registration', () {
    final infoPlistFile = File('ios/Runner/Info.plist');
    expect(infoPlistFile.existsSync(), isTrue,
        reason: 'ios/Runner/Info.plist must exist');

    final content = infoPlistFile.readAsStringSync();

    // Verify CFBundleURLTypes key exists
    expect(content.contains('<key>CFBundleURLTypes</key>'), isTrue);

    // Verify CFBundleTypeRole is Editor
    expect(
        content.contains('<key>CFBundleTypeRole</key>') &&
            content.contains('<string>Editor</string>'),
        isTrue);

    // Verify CFBundleURLSchemes contains meowgram
    expect(
        content.contains('<key>CFBundleURLSchemes</key>') &&
            content.contains('<string>meowgram</string>'),
        isTrue);
  });
}
