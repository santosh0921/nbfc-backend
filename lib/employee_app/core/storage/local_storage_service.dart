import 'package:shared_preferences/shared_preferences.dart';

/// Thin wrapper over SharedPreferences for non-sensitive local state
/// (theme mode, remember-me flag, last-used login identifier, etc).
class LocalStorageService {
  Future<SharedPreferences> get _prefs => SharedPreferences.getInstance();

  Future<String?> getString(String key) async => (await _prefs).getString(key);

  Future<bool> setString(String key, String value) async => (await _prefs).setString(key, value);

  Future<bool?> getBool(String key) async => (await _prefs).getBool(key);

  Future<bool> setBool(String key, bool value) async => (await _prefs).setBool(key, value);

  Future<bool> remove(String key) async => (await _prefs).remove(key);
}
