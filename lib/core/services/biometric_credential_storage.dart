import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Stores the MPIN in OS-encrypted secure storage (Android Keystore / iOS
/// Keychain via flutter_secure_storage) so a device that has opted into
/// fingerprint login can retrieve it after a successful biometric prompt
/// and use it to complete the exact same real `POST /auth/login` call
/// [AuthProvider.unlockWithMpin] already makes — biometric authentication
/// gates *access to* the stored MPIN, it never bypasses server-side
/// verification. This is a materially different (and stronger) storage
/// choice than [TokenStorage]'s SharedPreferences, deliberately: an MPIN
/// is a real credential, not a session token.
class BiometricCredentialStorage {
  BiometricCredentialStorage._();

  static const _storage = FlutterSecureStorage();
  static const _mpinKeyPrefix = 'biometric_mpin_';

  static Future<void> saveMpin(String mobile, String mpin) async {
    await _storage.write(key: '$_mpinKeyPrefix$mobile', value: mpin);
  }

  static Future<String?> readMpin(String mobile) async {
    return _storage.read(key: '$_mpinKeyPrefix$mobile');
  }

  static Future<void> clear(String mobile) async {
    await _storage.delete(key: '$_mpinKeyPrefix$mobile');
  }
}
