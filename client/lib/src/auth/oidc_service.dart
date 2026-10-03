import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:meowgram_client/src/auth/auth_state.dart';
import 'package:meowgram_client/src/auth/pkce_helper.dart';
import 'package:meowgram_client/src/config/app_config.dart';

/// Service responsible for executing OIDC discovery, PKCE authorization URL generation,
/// code-for-token exchange, and token refreshes against Authelia.
class OidcService {
  final http.Client _httpClient;

  String? _authorizationEndpoint;
  String? _tokenEndpoint;

  OidcService({http.Client? httpClient})
      : _httpClient = httpClient ?? http.Client();

  /// Discovers OIDC endpoints from `/.well-known/openid-configuration`
  /// with fallback to standard Authelia endpoints.
  Future<void> discoverEndpoints() async {
    if (_authorizationEndpoint != null && _tokenEndpoint != null) return;

    final issuer = AppConfig.autheliaIssuerUrl.replaceAll(RegExp(r'/+$'), '');
    final discoveryUrl = Uri.parse('$issuer/.well-known/openid-configuration');

    try {
      final response = await _httpClient
          .get(discoveryUrl)
          .timeout(const Duration(seconds: 5));
      if (response.statusCode == 200) {
        final data = jsonDecode(response.body) as Map<String, dynamic>;
        _authorizationEndpoint = data['authorization_endpoint'] as String?;
        _tokenEndpoint = data['token_endpoint'] as String?;
      }
    } catch (_) {
      // Fallback to standard Authelia OIDC endpoint paths if discovery endpoint times out
    }

    _authorizationEndpoint ??= '$issuer/api/oidc/authorization';
    _tokenEndpoint ??= '$issuer/api/oidc/token';
  }

  /// Builds the authorization URI with RFC 7636 PKCE query parameters.
  Future<Uri> buildAuthorizationUri({
    required PkcePair pkce,
    required String redirectUri,
  }) async {
    await discoverEndpoints();

    final baseAuthUri = Uri.parse(_authorizationEndpoint!);
    final queryParams = Map<String, String>.from(baseAuthUri.queryParameters)
      ..addAll({
        'client_id': AppConfig.autheliaClientId,
        'response_type': 'code',
        'scope': AppConfig.oidcScopes.join(' '),
        'redirect_uri': redirectUri,
        'state': pkce.state,
        'code_challenge': pkce.codeChallenge,
        'code_challenge_method': pkce.codeChallengeMethod,
        'nonce': pkce.nonce,
      });

    return baseAuthUri.replace(queryParameters: queryParams);
  }

  /// Exchanges an authorization code for an access token and refresh token using PKCE.
  ///
  /// Notice that no `client_secret` is transmitted, adhering strictly to the public client PKCE specification.
  Future<TokenData> exchangeCodeForToken({
    required String code,
    required String codeVerifier,
    required String redirectUri,
  }) async {
    await discoverEndpoints();

    final tokenUri = Uri.parse(_tokenEndpoint!);
    final response = await _httpClient.post(
      tokenUri,
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
        'Accept': 'application/json',
      },
      body: {
        'grant_type': 'authorization_code',
        'client_id': AppConfig.autheliaClientId,
        'code': code,
        'redirect_uri': redirectUri,
        'code_verifier': codeVerifier,
      },
    );

    if (response.statusCode != 200) {
      throw Exception(
          'Token exchange failed (${response.statusCode}): ${response.body}');
    }

    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return TokenData.fromJson(data);
  }

  /// Refreshes an expired access token using the stored refresh token.
  Future<TokenData> refreshToken({required String refreshToken}) async {
    await discoverEndpoints();

    final tokenUri = Uri.parse(_tokenEndpoint!);
    final response = await _httpClient.post(
      tokenUri,
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
        'Accept': 'application/json',
      },
      body: {
        'grant_type': 'refresh_token',
        'client_id': AppConfig.autheliaClientId,
        'refresh_token': refreshToken,
      },
    );

    if (response.statusCode != 200) {
      throw Exception(
          'Token refresh failed (${response.statusCode}): ${response.body}');
    }

    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return TokenData.fromJson(data);
  }
}
