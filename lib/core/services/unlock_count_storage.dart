import 'package:shared_preferences/shared_preferences.dart';

/// Counts successful MPIN unlocks on this device (not full phone+OTP
/// logins) so the app can offer "enable fingerprint login" after the
/// customer has typed their MPIN a few times in a row, rather than
/// pushing it on the very first re-open.
class UnlockCountStorage {
  UnlockCountStorage._();

  static const _key = 'mpin_unlock_count';
  static const _dismissedKey = 'biometric_offer_dismissed';

  static Future<int> read() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getInt(_key) ?? 0;
  }

  static Future<int> increment() async {
    final prefs = await SharedPreferences.getInstance();
    final next = (prefs.getInt(_key) ?? 0) + 1;
    await prefs.setInt(_key, next);
    return next;
  }

  /// The user can decline the biometric offer once and not be nagged
  /// again every single unlock — they can still turn it on later from
  /// Security settings.
  static Future<void> markOfferDismissed() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_dismissedKey, true);
  }

  static Future<bool> isOfferDismissed() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getBool(_dismissedKey) ?? false;
  }
}
