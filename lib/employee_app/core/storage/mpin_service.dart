import 'dart:convert';
import 'package:crypto/crypto.dart';
import 'secure_storage_service.dart';

/// Local-only "app lock" MPIN for quick re-entry into an already-valid
/// employee session, mirroring the pattern used by the customer app's own
/// MPIN unlock. This is deliberately NOT a backend auth mechanism: the real
/// authentication already happened via `/employee/login` (name + password)
/// and produced the JWT that [SecureStorageService] persists under
/// `auth_token`. The MPIN set here only gates whether *this device* shows
/// the dashboard again without re-typing name+password, the same way a
/// banking app might ask for a PIN/biometric to resume an already-logged-in
/// session. If the MPIN is forgotten, the fix is simply to fall back to a
/// normal name+password login — no backend call is ever made with the MPIN
/// itself, so there's no server-side concept of it to reset.
///
/// The MPIN is stored (via the platform keychain/keystore-backed
/// [SecureStorageService], the same place the JWT itself is kept) as a
/// salted SHA-256 hash rather than in the clear, as basic defense in depth
/// even though the underlying storage is already encrypted at rest.
class MpinService {
  MpinService(this._secureStorage);

  final SecureStorageService _secureStorage;

  String _key(String employeeCode) => 'mpin_hash_$employeeCode';

  String _hash(String employeeCode, String mpin) => sha256.convert(utf8.encode('mpin::$employeeCode::$mpin')).toString();

  Future<void> setMpin(String employeeCode, String mpin) => _secureStorage.write(_key(employeeCode), _hash(employeeCode, mpin));

  Future<bool> hasMpin(String employeeCode) async => (await _secureStorage.read(_key(employeeCode))) != null;

  Future<bool> verifyMpin(String employeeCode, String mpin) async {
    final stored = await _secureStorage.read(_key(employeeCode));
    if (stored == null) return false;
    return stored == _hash(employeeCode, mpin);
  }

  Future<void> clearMpin(String employeeCode) => _secureStorage.delete(_key(employeeCode));
}
