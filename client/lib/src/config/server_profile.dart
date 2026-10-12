import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:http/http.dart' as http;

/// Path every meowGram server answers without authentication (server
/// `handler.ClientConfigPath`). Fixed, because it's how a client learns the rest.
const String kClientConfigPath = '/api/client-config';

/// The server a domain-free (published) build talks to, as reported by that
/// server's `/api/client-config`. See runbook §10.11.
@immutable
class ServerProfile {
  /// Origin of the meowGram server, e.g. `https://meow.example.home.arpa`.
  final String baseUrl;
  final String issuer;
  final String clientId;
  final String wsPath;
  final String wsTicketPath;
  final String syncPath;
  final String pushDevicesPath;
  final String healthPath;

  const ServerProfile({
    required this.baseUrl,
    required this.issuer,
    required this.clientId,
    this.wsPath = '/ws',
    this.wsTicketPath = '/api/ws-ticket',
    this.syncPath = '/api/messages/sync',
    this.pushDevicesPath = '/api/push/devices',
    this.healthPath = '/healthz',
  });

  Uri get _base => Uri.parse(baseUrl);

  /// Host, plus the port when it isn't the scheme's default.
  String get domain => _base.hasPort && !_isDefaultPort(_base)
      ? '${_base.host}:${_base.port}'
      : _base.host;

  /// The issuer's host, shown on the login screen.
  String get issuerDomain => Uri.tryParse(issuer)?.authority ?? issuer;

  String get wsUrl => _base
      .replace(scheme: _base.scheme == 'https' ? 'wss' : 'ws', path: wsPath)
      .toString();

  String resolve(String path) => _base.replace(path: path).toString();

  /// Builds a profile from a `/api/client-config` response, rejecting anything
  /// a login could not safely use.
  factory ServerProfile.fromClientConfig(String baseUrl, Object? json) {
    if (json is! Map) {
      throw const ServerConfigException(
          'The server sent an unexpected answer.');
    }
    String path(String key, String fallback) {
      final value = json[key];
      return value is String && value.startsWith('/') ? value : fallback;
    }

    final issuer = (json['issuer'] as String? ?? '').replaceAll(RegExp(r'/+$'), '');
    final clientId = json['client_id'] as String? ?? '';
    final issuerUri = Uri.tryParse(issuer);
    if (issuerUri == null || !issuerUri.hasAuthority || clientId.isEmpty) {
      throw const ServerConfigException(
          'The server has no sign-in configured (issuer or client ID missing).');
    }
    if (issuerUri.scheme != 'https' && !isLoopback(issuerUri.host)) {
      throw const ServerConfigException(
          'The server\'s sign-in provider must use https://.');
    }
    return ServerProfile(
      baseUrl: baseUrl,
      issuer: issuer,
      clientId: clientId,
      wsPath: path('ws_path', '/ws'),
      wsTicketPath: path('ws_ticket_path', '/api/ws-ticket'),
      syncPath: path('sync_path', '/api/messages/sync'),
      pushDevicesPath: path('push_devices_path', '/api/push/devices'),
      healthPath: path('health_path', '/healthz'),
    );
  }

  Map<String, String> toJson() => {
        'base_url': baseUrl,
        'issuer': issuer,
        'client_id': clientId,
        'ws_path': wsPath,
        'ws_ticket_path': wsTicketPath,
        'sync_path': syncPath,
        'push_devices_path': pushDevicesPath,
        'health_path': healthPath,
      };

  static ServerProfile? fromJson(Map<String, dynamic> json) {
    final base = json['base_url'];
    if (base is! String || base.isEmpty) return null;
    try {
      return ServerProfile.fromClientConfig(base, json);
    } on ServerConfigException {
      return null;
    }
  }

  /// Turns what a person typed ("meow.example.home.arpa", a full URL, …) into
  /// the server's origin. Plain http is only accepted for this machine.
  static String normalizeAddress(String input) {
    var text = input.trim();
    if (text.isEmpty) {
      throw const ServerConfigException('Enter your meowGram server address.');
    }
    if (!text.contains('://')) {
      final host = Uri.tryParse('http://$text')?.host ?? '';
      text = '${isLoopback(host) ? 'http' : 'https'}://$text';
    }
    final uri = Uri.tryParse(text);
    if (uri == null ||
        uri.host.isEmpty ||
        (uri.scheme != 'https' && uri.scheme != 'http')) {
      throw const ServerConfigException(
          'That doesn\'t look like a server address.');
    }
    if (uri.scheme == 'http' && !isLoopback(uri.host)) {
      throw const ServerConfigException(
          'Use https:// for a server on another machine.');
    }
    final port = uri.hasPort && !_isDefaultPort(uri) ? ':${uri.port}' : '';
    return '${uri.scheme}://${uri.host}$port';
  }

  static bool isLoopback(String host) =>
      host == 'localhost' || host == '127.0.0.1' || host == '::1';

  static bool _isDefaultPort(Uri uri) =>
      (uri.scheme == 'https' && uri.port == 443) ||
      (uri.scheme == 'http' && uri.port == 80);

  @override
  bool operator ==(Object other) =>
      other is ServerProfile && mapEquals(other.toJson(), toJson());

  @override
  int get hashCode => Object.hashAll(toJson().values);
}

class ServerConfigException implements Exception {
  final String message;
  const ServerConfigException(this.message);
  @override
  String toString() => message;
}

/// Asks a server for its `/api/client-config`.
Future<ServerProfile> fetchServerProfile(
  String address, {
  http.Client? client,
  Duration timeout = const Duration(seconds: 10),
}) async {
  final base = ServerProfile.normalizeAddress(address);
  final httpClient = client ?? http.Client();
  try {
    final res = await httpClient
        .get(Uri.parse('$base$kClientConfigPath'),
            headers: const {'Accept': 'application/json'})
        .timeout(timeout);
    if (res.statusCode != 200) {
      throw ServerConfigException(
          'That address didn\'t answer like a meowGram server (HTTP ${res.statusCode}).');
    }
    final Object? json;
    try {
      json = jsonDecode(res.body);
    } on FormatException {
      throw const ServerConfigException(
          'That address didn\'t answer like a meowGram server.');
    }
    return ServerProfile.fromClientConfig(base, json);
  } on TimeoutException {
    throw const ServerConfigException('The server didn\'t answer in time.');
  } on http.ClientException catch (e) {
    throw ServerConfigException('Couldn\'t reach the server: ${e.message}');
  } finally {
    if (client == null) httpClient.close();
  }
}

/// Keeps the chosen server next to the tokens (also readable by the
/// background isolate).
class ServerProfileStore {
  static const _key = 'meowgram_server_profile';
  final FlutterSecureStorage _storage;

  ServerProfileStore({FlutterSecureStorage? storage})
      : _storage = storage ?? const FlutterSecureStorage();

  Future<ServerProfile?> read() async {
    try {
      final raw = await _storage.read(key: _key);
      if (raw == null || raw.isEmpty) return null;
      final json = jsonDecode(raw);
      return json is Map<String, dynamic> ? ServerProfile.fromJson(json) : null;
    } catch (_) {
      return null;
    }
  }

  Future<void> write(ServerProfile profile) =>
      _storage.write(key: _key, value: jsonEncode(profile.toJson()));

  Future<void> clear() => _storage.delete(key: _key);
}
