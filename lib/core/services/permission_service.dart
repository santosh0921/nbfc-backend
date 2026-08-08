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

  static Future<void> openSettings() => openAppSettings();
}
