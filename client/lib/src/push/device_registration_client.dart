import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:meowgram_client/src/config/app_config.dart';

class DeviceRegistrationClient {
  final http.Client _client;
  final String? endpoint;
  DeviceRegistrationClient({http.Client? client, this.endpoint})
    : _client = client ?? http.Client();

  Future<void> register(String token, String bearer) =>
      _send('PUT', token, bearer);
  Future<void> unregister(String token, String bearer) =>
      _send('DELETE', token, bearer);

  Future<void> _send(String method, String token, String bearer) async {
    if (token.isEmpty || bearer.isEmpty) {
      throw StateError('Signed-in device required');
    }
    final request =
        http.Request(method, Uri.parse(endpoint ?? AppConfig.pushDeviceUrl))
          ..headers.addAll({
            'Authorization': 'Bearer $bearer',
            'Content-Type': 'application/json',
          })
          ..body = jsonEncode({'token': token});
    final response = await _client
        .send(request)
        .timeout(const Duration(seconds: 10));
    await response.stream.drain<void>().timeout(const Duration(seconds: 10));
    if (response.statusCode != 204) {
      throw StateError('Device registration failed (${response.statusCode})');
    }
  }
}
