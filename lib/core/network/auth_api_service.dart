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

  /// Returns whether this mobile already has an account with an MPIN set
  /// — the app uses this to route a returning user straight to "enter
  /// your existing MPIN" instead of "create a new MPIN" after logout.
  static Future<bool> verifyOtp(String mobile, String otp) async {
    final res = await ApiClient.post('/auth/verify-otp', {'mobile': mobile, 'otp': otp});
    return res['accountExists'] as bool? ?? false;
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

  /// Logged-in path: the customer knows their current MPIN. Backend
  /// verifies it server-side before accepting the new one.
  static Future<void> changeMpin(String token, String currentMpin, String newMpin, String confirmMpin) async {
    await ApiClient.put('/auth/change-mpin', {
      'current_mpin': currentMpin,
      'new_mpin': newMpin,
      'confirm_mpin': confirmMpin,
    }, token: token);
  }

  /// Forgot-MPIN path: no token yet — gated by the same OTP verification
  /// as sign-up (sendOtp/verifyOtp above), then the backend resets the
  /// MPIN for that mobile once verified.
  static Future<void> forgotMpin(String mobile, String newMpin, String confirmMpin) async {
    await ApiClient.post('/auth/forgot-mpin', {
      'mobile': mobile,
      'new_mpin': newMpin,
      'confirm_mpin': confirmMpin,
    });
  }
}
