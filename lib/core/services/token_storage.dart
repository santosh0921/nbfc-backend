import 'package:shared_preferences/shared_preferences.dart';

/// Persists the JWT issued by `POST /auth/login` across the session.
/// Kept in SharedPreferences (matching the rest of the app's lightweight
/// persistence) rather than secure storage — acceptable here since this
/// is a demo/dev backend, not a production credential store.
class TokenStorage {
  TokenStorage._();

  static const _tokenKey = 'auth_jwt_token';
  static const _mobileKey = 'auth_mobile';

  static Future<void> save({required String token, required String mobile}) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_tokenKey, token);
    await prefs.setString(_mobileKey, mobile);
  }

  static Future<String?> readToken() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_tokenKey);
  }

  static Future<String?> readMobile() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_mobileKey);
  }

  static Future<void> clear() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_tokenKey);
    await prefs.remove(_mobileKey);
  }
}
