import 'package:shared_preferences/shared_preferences.dart';

/// Historical marker of whether the customer flow has ever completed a
/// real login on this device. No longer consulted for navigation — the
/// app always starts at the customer entry point (see SplashScreen) so
/// the back button always has somewhere to go, rather than jumping
/// straight into whichever "side" was last used.
class LastRoleStorage {
  LastRoleStorage._();

  static const _roleKey = 'last_role';

  static Future<void> saveCustomer() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_roleKey, 'customer');
  }
}
