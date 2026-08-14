import 'package:shared_preferences/shared_preferences.dart';

/// Tracks which "big" notifications (Sanction/Disbursement Letter ready)
/// have already been shown as a full-screen takeover, so a customer only
/// ever sees the interstitial once per notification — not every time the
/// home screen reloads while it's still unread.
class ShownBigNotificationsStore {
  ShownBigNotificationsStore._();

  static const _key = 'shown_big_notification_ids';

  static Future<bool> hasShown(int notificationId) async {
    final prefs = await SharedPreferences.getInstance();
    final shown = prefs.getStringList(_key) ?? const [];
    return shown.contains('$notificationId');
  }

  static Future<void> markShown(int notificationId) async {
    final prefs = await SharedPreferences.getInstance();
    final shown = prefs.getStringList(_key) ?? const [];
    if (shown.contains('$notificationId')) return;
    await prefs.setStringList(_key, [...shown, '$notificationId']);
  }
}
