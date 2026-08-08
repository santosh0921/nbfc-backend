import 'package:flutter/foundation.dart';
import '../config/app_config.dart';
import '../network/api_exception.dart';
import '../network/auth_api_service.dart';
import '../network/kyc_api_service.dart';
import '../services/token_storage.dart';

/// App-wide session state. Singleton (mirroring [ThemeProvider]'s scope,
/// but session state needs to be readable by the router's `redirect`
/// callback outside the widget tree, hence the static instance) so
/// `GoRouter`'s `refreshListenable` can react to login/logout without
/// needing a `BuildContext`.
///
/// Can run against the real Go/Gin backend in `cmd/api`, or fully mocked
/// locally — see [AppConfig.useMockBackend]. When mocked, [requestOtp]
/// always "succeeds" and returns a fixed demo OTP, [verifyOtp] accepts
/// any complete 6-digit code, and [createMpinAndLogin] just simulates a
/// short delay before granting a session — no network calls at all.
class AuthProvider extends ChangeNotifier {
  AuthProvider._();

  static final AuthProvider instance = AuthProvider._();

  bool hasSession = false;
  String? pendingPhoneNumber;
  String? token;
  bool biometricEnabled = false;

  // Collected on the post-OTP Registration screen, saved to the backend
  // profile right after the first successful login (real-backend mode
  // only).
  String? pendingFullName;
  String? pendingEmail;
  String? pendingDob;

  /// Step 1: request an OTP for this mobile number. Returns the OTP
  /// itself in dev mode (the backend echoes it back until an SMS
  /// provider is wired up server-side) so the UI can prefill/display it.
  Future<String?> requestOtp(String phoneNumber) async {
    pendingPhoneNumber = phoneNumber;
    if (AppConfig.useMockBackend) {
      await Future<void>.delayed(const Duration(milliseconds: 500));
      return '123456';
    }
    return AuthApiService.sendOtp(phoneNumber);
  }

  /// Step 2: verify the OTP the user entered.
  Future<bool> verifyOtp(String code) async {
    final mobile = pendingPhoneNumber;
    if (mobile == null) return false;
    if (AppConfig.useMockBackend) {
      await Future<void>.delayed(const Duration(milliseconds: 700));
      return code.length == 6;
    }
    try {
      await AuthApiService.verifyOtp(mobile, code);
      return true;
    } on ApiException {
      return false;
    }
  }

  /// Step 3: set (or reset) the MPIN, then immediately log in with it to
  /// obtain a session — in real-backend mode this is a JWT from
  /// `/auth/login` (create-mpin doesn't itself return a token); in mock
  /// mode it's just a local flag flip.
  Future<void> createMpinAndLogin(String mpin, String confirmMpin) async {
    final mobile = pendingPhoneNumber;
    if (mobile == null) throw ApiException('Missing mobile number — please restart login.');

    if (AppConfig.useMockBackend) {
      await Future<void>.delayed(const Duration(milliseconds: 500));
      hasSession = true;
      notifyListeners();
      return;
    }

    await AuthApiService.createMpin(mobile, mpin, confirmMpin);
    await _loginAndPersist(mobile, mpin);
  }

  Future<void> _loginAndPersist(String mobile, String mpin) async {
    final result = await AuthApiService.login(mobile, mpin);
    token = result.token;
    biometricEnabled = result.biometricEnabled;
    hasSession = true;
    await TokenStorage.save(token: result.token, mobile: mobile);
    notifyListeners();
    await _saveRegisteredProfile();
  }

  /// Saves the name/email/DOB collected on the Registration screen to
  /// the backend profile now that we have a JWT. Best-effort — a failure
  /// here (e.g. profile already exists from a prior run) shouldn't block
  /// getting into the app.
  Future<void> _saveRegisteredProfile() async {
    final t = token;
    final name = pendingFullName?.trim();
    if (t == null || name == null || name.isEmpty) return;
    final parts = name.split(RegExp(r'\s+'));
    try {
      await KycApiService.createProfile(
        t,
        firstName: parts.first,
        lastName: parts.length > 1 ? parts.sublist(1).join(' ') : parts.first,
        email: pendingEmail?.trim() ?? '',
        dateOfBirth: pendingDob ?? '',
        gender: 'Prefer not to say',
        maritalStatus: 'Prefer not to say',
        occupation: 'Not specified',
      );
    } on ApiException {
      // Non-fatal — e.g. profile already exists on a repeat login.
    }
  }

  Future<void> setBiometricEnabled(bool enabled) async {
    biometricEnabled = enabled;
    notifyListeners();
    if (AppConfig.useMockBackend) return;
    final t = token;
    if (t == null) return;
    await AuthApiService.setBiometricEnabled(t, enabled);
  }

  void logout() {
    hasSession = false;
    pendingPhoneNumber = null;
    token = null;
    biometricEnabled = false;
    TokenStorage.clear();
    notifyListeners();
  }
}
