import 'package:flutter/foundation.dart';
import '../config/app_config.dart';
import '../network/api_exception.dart';
import '../network/auth_api_service.dart';
import '../network/kyc_api_service.dart';
import '../services/last_role_storage.dart';
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
  String? pendingFirstName;
  String? pendingMiddleName;
  String? pendingLastName;
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
      await LastRoleStorage.saveCustomer();
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
    await LastRoleStorage.saveCustomer();
    notifyListeners();
    // Deliberately awaited (not fire-and-forget) so a 400 — most notably
    // the name-collision guard on `POST /auth/profile` — propagates back
    // up through [createMpinAndLogin] to the Create MPIN screen, which is
    // the earliest point in the flow a JWT exists to even attempt this
    // call. That lets the caller keep the user on that screen with an
    // actionable error instead of dropping them into the app first and
    // surfacing it later. Other failures (e.g. profile already exists
    // from a prior run) stay non-fatal.
    await _saveRegisteredProfile();
  }

  /// Saves the name/email/DOB collected on the Registration screen to
  /// the backend profile now that we have a JWT.
  ///
  /// Rethrows on HTTP 400 (invalid input, or the name-collision guard —
  /// "An account with this exact name already exists...") since those are
  /// actionable by the user right now. Swallows everything else (e.g. 409
  /// profile-already-exists on a repeat login) — best-effort, shouldn't
  /// block getting into the app.
  Future<void> _saveRegisteredProfile() async {
    final t = token;
    final first = pendingFirstName?.trim();
    final last = pendingLastName?.trim();
    if (t == null || first == null || first.isEmpty || last == null || last.isEmpty) return;
    try {
      await KycApiService.createProfile(
        t,
        firstName: first,
        middleName: pendingMiddleName?.trim(),
        lastName: last,
        email: pendingEmail?.trim() ?? '',
        dateOfBirth: pendingDob ?? '',
        gender: 'Prefer not to say',
        maritalStatus: 'Prefer not to say',
        occupation: 'Not specified',
      );
    } on ApiException catch (e) {
      if (e.statusCode == 400) rethrow;
      // Non-fatal — e.g. profile already exists on a repeat login.
    }
  }

  /// Checks whether this device has a saved token/mobile from a previous
  /// successful login (see [TokenStorage]). Used by [SplashScreen] to
  /// decide whether a fresh app process should go through the full
  /// phone+OTP+MPIN flow again or can offer the lightweight MPIN-unlock
  /// screen instead. Deliberately does NOT set [hasSession] itself — the
  /// saved token could be stale/expired, so it's only trusted after a
  /// fresh `/auth/login` call succeeds (see [unlockWithMpin]).
  Future<String?> savedMobileForUnlock() async {
    if (AppConfig.useMockBackend) return null;
    final savedToken = await TokenStorage.readToken();
    final savedMobile = await TokenStorage.readMobile();
    if (savedToken == null || savedMobile == null) return null;
    return savedMobile;
  }

  /// Quick re-login for the MPIN-unlock screen: same real server-side
  /// verification as the original login (`POST /auth/login`), just
  /// skipping the phone+OTP steps since this device already proved it
  /// once. On success this is functionally identical to a normal login.
  Future<void> unlockWithMpin(String mpin) async {
    final mobile = await TokenStorage.readMobile();
    if (mobile == null) throw ApiException('No saved account on this device — please log in again.');
    pendingPhoneNumber = mobile;
    await _loginAndPersist(mobile, mpin);
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
