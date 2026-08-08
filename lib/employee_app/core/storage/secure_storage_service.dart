import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Sensitive session data (auth token, employee id) — backed by platform
/// keychain/keystore. Ready to hold the JWT issued by the future Go backend.
class SecureStorageService {
  final _storage = const FlutterSecureStorage();

  Future<String?> read(String key) => _storage.read(key: key);

  Future<void> write(String key, String value) => _storage.write(key: key, value: value);

  Future<void> delete(String key) => _storage.delete(key: key);

  Future<void> deleteAll() => _storage.deleteAll();
}
