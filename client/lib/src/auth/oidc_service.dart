import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:jose/jose.dart';
import 'package:meowgram_client/src/auth/auth_state.dart';
import 'package:meowgram_client/src/auth/pkce_helper.dart';
import 'package:meowgram_client/src/config/app_config.dart';

/// Thrown when the token endpoint definitively rejects a grant (HTTP 400/401,
/// e.g. `invalid_grant` for an expired or revoked refresh token). Network errors
/// and timeouts are *not* rejections and must not end the session.
class OidcTokenRejectedException implements Exception {
  final int statusCode;
  final String body;
  const OidcTokenRejectedException(this.statusCode, this.body);

  @override
  String toString() => 'Token request rejected ($statusCode)';
}

/// Timeout applied to every request against Authelia.
const Duration kOidcRequestTimeout = Duration(seconds: 15);

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

    final discoveryUrl = Uri.parse(AppConfig.autheliaDiscoveryUrl);

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
      // Fallback to configured Authelia OIDC endpoint paths if discovery endpoint times out
    }

    _authorizationEndpoint ??= AppConfig.autheliaAuthorizationEndpoint;
    _tokenEndpoint ??= AppConfig.autheliaTokenEndpoint;
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
    required String expectedNonce,
    required String redirectUri,
  }) async {
    await discoverEndpoints();

    final tokens = await _tokenRequest({
      'grant_type': 'authorization_code',
      'client_id': AppConfig.autheliaClientId,
      'code': code,
      'redirect_uri': redirectUri,
      'code_verifier': codeVerifier,
    });
    await _validateIdToken(tokens.idToken, expectedNonce);
    return tokens;
  }

  /// Validate the signed ID token before storing a newly authenticated session.
  /// Keys come only from the configured issuer's JWKS, never from token headers.
  Future<void> _validateIdToken(String? token, String nonce) async {
    if (token == null || token.isEmpty || nonce.isEmpty) {
      throw const FormatException(
        'Sign-in response is missing its ID token or nonce.',
      );
    }
    try {
      final response = await _httpClient
          .get(Uri.parse(AppConfig.autheliaJwksUrl))
          .timeout(kOidcRequestTimeout);
      if (response.statusCode != 200) {
        throw const FormatException('Signing keys unavailable.');
      }
      final keys = JsonWebKeyStore();
      final document = jsonDecode(response.body) as Map<String, dynamic>;
      for (final key in document['keys'] as List<dynamic>) {
        final json = key as Map<String, dynamic>;
        if (json['kty'] == 'RSA') keys.addKey(JsonWebKey.fromJson(json));
      }
      final signed = JsonWebSignature.fromCompactSerialization(token);
      final payload = await signed.getPayload(
        keys,
        allowedAlgorithms: ['RS256'],
      );
      final claims = jsonDecode(payload.stringContent) as Map<String, dynamic>;
      final audience = claims['aud'];
      final audiences = audience is String ? [audience] : audience;
      final now = DateTime.now().toUtc().millisecondsSinceEpoch ~/ 1000;
      if (claims['iss'] != AppConfig.autheliaIssuerUrl ||
          audiences is! List ||
          !audiences.contains(AppConfig.autheliaClientId) ||
          (audiences.length > 1 &&
              claims['azp'] != AppConfig.autheliaClientId) ||
          (claims.containsKey('azp') &&
              claims['azp'] != AppConfig.autheliaClientId) ||
          claims['sub'] is! String ||
          (claims['sub'] as String).isEmpty ||
          claims['exp'] is! int ||
          (claims['exp'] as int) <= now ||
          claims['iat'] is! int ||
          (claims['iat'] as int) > now + 60 ||
          (claims.containsKey('nbf') &&
              (claims['nbf'] is! int || (claims['nbf'] as int) > now + 60)) ||
          claims['nonce'] != nonce) {
        throw const FormatException('Invalid ID token claims.');
      }
    } catch (_) {
      // Parser and crypto exceptions may contain token material. Do not expose it.
      throw const FormatException(
        'Sign-in ID token validation failed. Please sign in again.',
      );
    }
  }

  /// Refreshes an expired access token using the stored refresh token.
  Future<TokenData> refreshToken({required String refreshToken}) async {
    await discoverEndpoints();

    final refreshed = await _tokenRequest({
      'grant_type': 'refresh_token',
      'client_id': AppConfig.autheliaClientId,
      'refresh_token': refreshToken,
    });
    // Authelia may omit refresh_token when rotation is disabled: keep the old one.
    if (refreshed.refreshToken == null || refreshed.refreshToken!.isEmpty) {
      return TokenData(
        accessToken: refreshed.accessToken,
        idToken: refreshed.idToken,
        refreshToken: refreshToken,
        tokenType: refreshed.tokenType,
        expiresAt: refreshed.expiresAt,
      );
    }
    return refreshed;
  }

  Future<TokenData> _tokenRequest(Map<String, String> body) async {
    final response = await _httpClient
        .post(
          Uri.parse(_tokenEndpoint!),
          headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
            'Accept': 'application/json',
          },
          body: body,
        )
        .timeout(kOidcRequestTimeout);

    if (response.statusCode == 400 || response.statusCode == 401) {
      throw OidcTokenRejectedException(response.statusCode, response.body);
    }
    if (response.statusCode != 200) {
      throw Exception('Token request failed (${response.statusCode})');
    }

    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return TokenData.fromJson(data);
  }
}
