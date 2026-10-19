import 'dart:async';

import 'package:flutter/foundation.dart';
import '../config/app_config.dart';
import '../network/api_client.dart';
import '../network/api_exception.dart';
import '../network/auth_api_service.dart';
import '../network/kyc_api_service.dart';
import '../services/biometric_credential_storage.dart';
import '../services/last_role_storage.dart';
import '../services/push_notification_service.dart';
import '../services/token_storage.dart';
import '../services/unlock_count_storage.dart';

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

  /// This customer's own backend user id (models.User.ID / Customer UUID)
  /// — set on every real login path via [_loginAndPersist], since
  /// `/auth/login`'s response already includes it. Needed so the app can
  /// listen for an employee/admin-initiated video call on this customer's
  /// own `kyc-<customerId>` signaling room (see IncomingCallListener),
  /// distinct from the loan-id-keyed room used for loan-verification calls.
  String? customerId;

  /// Set by [verifyOtp] — whether [pendingPhoneNumber] already has an
  /// account with an MPIN set. The OTP screen uses this to route a
  /// returning user (logged out, logging back in with the same number)
  /// straight to "enter your existing MPIN" instead of "create a new
  /// MPIN", which would otherwise silently overwrite their real one.
  bool pendingAccountExists = false;

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

  /// Step 2: verify the OTP the user entered. Also records whether this
  /// mobile already has an account (see [pendingAccountExists]).
  Future<bool> verifyOtp(String code) async {
    final mobile = pendingPhoneNumber;
    if (mobile == null) return false;
    if (AppConfig.useMockBackend) {
      await Future<void>.delayed(const Duration(milliseconds: 700));
      pendingAccountExists = false;
      return code.length == 6;
    }
    try {
      pendingAccountExists = await AuthApiService.verifyOtp(mobile, code);
      return true;
    } on ApiException {
      return false;
    }
  }

  /// Returning-user path: mobile+OTP already re-verified this session,
  /// the account is known to already have an MPIN (see
  /// [pendingAccountExists]) — this just logs in with it, the same real
  /// server-verified call as every other MPIN path. Distinct from
  /// [createMpinAndLogin], which would overwrite the existing MPIN.
  Future<void> loginWithMpin(String mobile, String mpin) async {
    pendingPhoneNumber = mobile;
    await _loginAndPersist(mobile, mpin);
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
    customerId = result.userId;
    biometricEnabled = result.biometricEnabled;
    hasSession = true;
    await TokenStorage.save(token: result.token, mobile: mobile);
    await LastRoleStorage.saveCustomer();
    notifyListeners();
    PushNotificationService.instance.setRegisterCallback(
      (fcmToken) => ApiClient.post('/device-token', {'token': fcmToken, 'platform': 'android'}, token: token),
    );
    // Fire-and-forget — a customer who's already inside the app matters
    // more than this completing first, and it's harmless if it fails
    // (retried on next login or the next onTokenRefresh).
    unawaited(PushNotificationService.instance.registerTokenForCurrentUser());
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
  /// "An account with this exact name already exists...") and on 409
  /// (profile already exists) since both are actionable/worth surfacing.
  /// The 409 case matters: `pendingAccountExists` (set from `/verify-otp`)
  /// only checks whether an MPIN was ever set — a mobile number that
  /// already has a real profile but was never fully self-registered (e.g.
  /// an employee captured it while filing a loan on the customer's
  /// behalf) still reports `accountExists = false`, routes through the
  /// full Registration screen, and accepts *any* typed name there. Before
  /// this fix, that mismatched name was silently discarded here and the
  /// customer was logged straight into the pre-existing account with no
  /// indication anything was off — "I typed a different name and it still
  /// let me in" was a real, reproducible report, not user error.
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
      if (e.statusCode == 400 || e.statusCode == 409) rethrow;
      // Genuinely non-fatal for any other status — shouldn't block
      // getting into the app.
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
    await UnlockCountStorage.increment();
  }

  /// Fingerprint/face unlock: gates access to the MPIN saved in secure
  /// storage behind the caller's own biometric prompt (the caller —
  /// [MpinUnlockPage] — runs `LocalAuthentication().authenticate()` before
  /// calling this), then completes the exact same server-verified login
  /// as typing the MPIN by hand. Throws if biometric login was never
  /// enabled on this device, or the stored credential is missing (e.g.
  /// cleared by a MPIN reset) — the caller falls back to manual MPIN entry.
  Future<void> unlockWithBiometric() async {
    final mobile = await TokenStorage.readMobile();
    if (mobile == null) throw ApiException('No saved account on this device — please log in again.');
    final mpin = await BiometricCredentialStorage.readMpin(mobile);
    if (mpin == null) {
      throw ApiException('Fingerprint login isn\'t set up on this device — enter your MPIN.');
    }
    pendingPhoneNumber = mobile;
    await _loginAndPersist(mobile, mpin);
    await UnlockCountStorage.increment();
  }

  /// Offer "enable fingerprint login" after a few manual MPIN unlocks in a
  /// row, not on the very first one — and never again once the customer
  /// has enabled it or explicitly dismissed the offer.
  Future<bool> shouldOfferBiometricEnrollment() async {
    if (biometricEnabled) return false;
    if (await UnlockCountStorage.isOfferDismissed()) return false;
    final count = await UnlockCountStorage.read();
    return count >= 3;
  }

  Future<void> dismissBiometricOffer() => UnlockCountStorage.markOfferDismissed();

  /// Enables biometric login: stores the MPIN the customer just proved
  /// they know (they either just typed it to unlock, or just set it) in
  /// secure storage, and flips the server-side preference flag.
  Future<void> enableBiometricLogin(String mpin) async {
    final mobile = await TokenStorage.readMobile() ?? pendingPhoneNumber;
    if (mobile == null) throw ApiException('Missing account — please log in again.');
    await BiometricCredentialStorage.saveMpin(mobile, mpin);
    await setBiometricEnabled(true);
  }

  Future<void> disableBiometricLogin() async {
    final mobile = await TokenStorage.readMobile();
    if (mobile != null) await BiometricCredentialStorage.clear(mobile);
    await setBiometricEnabled(false);
  }

  /// Logged-in path — the customer knows their current MPIN, backend
  /// verifies it server-side before accepting the new one. Keeps the
  /// biometric-stored copy in sync so fingerprint login doesn't silently
  /// start failing with the old MPIN.
  Future<void> changeMpin(String currentMpin, String newMpin, String confirmMpin) async {
    final t = token;
    if (t == null) throw ApiException('Not logged in.');
    await AuthApiService.changeMpin(t, currentMpin, newMpin, confirmMpin);
    if (biometricEnabled) {
      final mobile = await TokenStorage.readMobile();
      if (mobile != null) await BiometricCredentialStorage.saveMpin(mobile, newMpin);
    }
  }

  /// Forgot-MPIN path — no token yet, gated by the same OTP verification
  /// as sign-up. Disables biometric login afterwards: the stored MPIN is
  /// now stale, and re-enabling requires the customer to prove the new
  /// MPIN once more, same as any other credential rotation.
  Future<void> forgotMpin(String mobile, String newMpin, String confirmMpin) async {
    await AuthApiService.forgotMpin(mobile, newMpin, confirmMpin);
    await BiometricCredentialStorage.clear(mobile);
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
    customerId = null;
    biometricEnabled = false;
    TokenStorage.clear();
    notifyListeners();
  }
}
