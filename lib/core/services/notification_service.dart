import 'package:flutter_local_notifications/flutter_local_notifications.dart';

/// Thin wrapper around flutter_local_notifications: initializes the
/// plugin once and exposes a single [show] call for instant local
/// notifications (EMI reminders, application/payment confirmations).
/// This is a real, working notification — it appears in the device's
/// notification tray — not a mock. There's no backend/FCM in this app,
/// so it's local-only rather than a true remote push.
class NotificationService {
  NotificationService._();

  static final NotificationService instance = NotificationService._();

  final FlutterLocalNotificationsPlugin _plugin = FlutterLocalNotificationsPlugin();
  bool _initialized = false;
  int _nextId = 1000;

  static const _channel = AndroidNotificationDetails(
    'nbfc_premium_general',
    'NBFC Premium',
    channelDescription: 'Loan, payment, and application updates',
    importance: Importance.high,
    priority: Priority.high,
  );

  Future<void> init() async {
    if (_initialized) return;
    const androidInit = AndroidInitializationSettings('@mipmap/ic_launcher');
    const settings = InitializationSettings(android: androidInit);
    await _plugin.initialize(settings);
    _initialized = true;
  }

  Future<void> show({required String title, required String body}) async {
    if (!_initialized) await init();
    await _plugin.show(
      _nextId++,
      title,
      body,
      const NotificationDetails(android: _channel),
    );
  }
}
