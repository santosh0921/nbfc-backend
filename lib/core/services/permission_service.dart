import 'package:permission_handler/permission_handler.dart';

/// Real runtime permission requests (Android's actual system dialogs) for
/// notifications and location — used by [AppPermissionsScreen] and on
/// first login. Wraps permission_handler so the rest of the app only
/// deals with a simple granted/denied boolean.
class PermissionService {
  PermissionService._();

  static Future<bool> isNotificationGranted() async => Permission.notification.isGranted;

  static Future<bool> requestNotification() async {
    final status = await Permission.notification.request();
    return status.isGranted;
  }

  static Future<bool> isLocationGranted() async => Permission.location.isGranted;

  static Future<bool> requestLocation() async {
    final status = await Permission.location.request();
    return status.isGranted;
  }

  /// Whether the OS is currently allowed to kill this app in the
  /// background / throttle its wake-ups. Notification permission alone
  /// does not guarantee a push actually arrives while the app isn't in
  /// the foreground — on Vivo, Oppo, Xiaomi and similar OEMs' aggressive
  /// battery managers, an app that isn't exempted from battery
  /// optimization gets its process frozen or FCM wake-ups dropped
  /// entirely, so notifications only ever show up while the app happens
  /// to already be open. That's a device-level restriction, not something
  /// server-side push code can work around — this exemption is the actual
  /// fix.
  static Future<bool> isIgnoringBatteryOptimizations() async => Permission.ignoreBatteryOptimizations.isGranted;

  static Future<bool> requestIgnoreBatteryOptimizations() async {
    final status = await Permission.ignoreBatteryOptimizations.request();
    return status.isGranted;
  }

  static Future<void> openSettings() => openAppSettings();
}
