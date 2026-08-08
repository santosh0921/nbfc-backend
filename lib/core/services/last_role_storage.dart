import 'package:shared_preferences/shared_preferences.dart';

/// Remembers which "side" of the app (customer vs. employee, and which
/// employee module) was last actively used, so a fresh app launch can
/// route straight back into it instead of always defaulting to the
/// customer login screen. Written on every successful login; read once
/// by [SplashScreen] at startup.
class LastRoleStorage {
  LastRoleStorage._();

  static const _roleKey = 'last_role';
  static const _moduleKey = 'last_employee_module';

  static Future<void> saveCustomer() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_roleKey, 'customer');
  }

  /// [module] is `'verification'` or `'recovery'`.
  static Future<void> saveEmployee(String module) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_roleKey, 'employee');
    await prefs.setString(_moduleKey, module);
  }

  /// Returns `('customer', null)`, `('employee', 'verification'|'recovery')`,
  /// or `(null, null)` if nothing has ever been saved (first-ever launch).
  static Future<({String? role, String? module})> read() async {
    final prefs = await SharedPreferences.getInstance();
    return (role: prefs.getString(_roleKey), module: prefs.getString(_moduleKey));
  }
}
