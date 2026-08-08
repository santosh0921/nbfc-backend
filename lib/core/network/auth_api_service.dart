import 'api_client.dart';

/// Wraps the backend's public `/auth/*` endpoints (send-otp, verify-otp,
/// create-mpin, login) and the protected ones behind the bearer token
/// (profile, biometric toggle).
class AuthApiService {
  AuthApiService._();

  /// Dev-mode only: the backend echoes the generated OTP back in the
  /// response until an SMS provider is wired up server-side.
  static Future<String?> sendOtp(String mobile) async {
    final res = await ApiClient.post('/auth/send-otp', {'mobile': mobile});
    return res['otp'] as String?;
  }

  static Future<void> verifyOtp(String mobile, String otp) async {
    await ApiClient.post('/auth/verify-otp', {'mobile': mobile, 'otp': otp});
  }

  static Future<void> createMpin(String mobile, String mpin, String confirmMpin) async {
    await ApiClient.post('/auth/create-mpin', {
      'mobile': mobile,
      'mpin': mpin,
      'confirm_mpin': confirmMpin,
    });
  }

  /// Returns the JWT and basic user info on success.
  static Future<({String token, String userId, bool biometricEnabled})> login(String mobile, String mpin) async {
    final res = await ApiClient.post('/auth/login', {'mobile': mobile, 'mpin': mpin});
    final user = res['user'] as Map<String, dynamic>;
    return (
      token: res['token'] as String,
      userId: user['id'] as String,
      biometricEnabled: user['biometric_enabled'] as bool? ?? false,
    );
  }

  static Future<Map<String, dynamic>> getProfile(String token) async {
    return ApiClient.get('/auth/profile', token: token);
  }

  static Future<void> setBiometricEnabled(String token, bool enabled) async {
    await ApiClient.put('/auth/biometric', {'enabled': enabled}, token: token);
  }
}
