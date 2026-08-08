import 'package:hive_flutter/hive_flutter.dart';
import 'core/config/app_config.dart';
import 'core/di/service_locator.dart';

/// Ported from onefin_employee's original `bootstrap()`, minus the
/// `runApp` call — NBFC already owns the single running `runApp` for the
/// process. This just does the one-time Hive/ServiceLocator init so
/// [OnefinEmployeeApp] can be mounted as a normal widget from NBFC's own
/// router whenever the Employee role is entered. Safe to call more than
/// once (e.g. re-entering the Employee section after logging out) —
/// guarded so init only actually runs the first time.
bool _initialized = false;

Future<void> ensureEmployeeAppInitialized() async {
  if (_initialized) return;
  AppConfig.init(AppConfig.development);
  await Hive.initFlutter();
  await ServiceLocator.instance.init();
  _initialized = true;
}
