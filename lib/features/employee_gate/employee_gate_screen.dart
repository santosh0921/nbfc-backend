import 'package:flutter/material.dart';
import '../../employee_app/app.dart';
import '../../employee_app/bootstrap.dart';

/// NBFC-side entry point into the Employee role. Runs the one-time Hive/
/// ServiceLocator init the ported onefin_employee app needs, then mounts
/// its own [OnefinEmployeeApp] widget — which owns its own internal
/// `MaterialApp.router`, navigation stack, and auth state from that
/// point on, completely independent of NBFC's customer-side router.
class EmployeeGateScreen extends StatelessWidget {
  const EmployeeGateScreen({super.key, this.module = EmployeeAppModule.verification});

  final EmployeeAppModule module;

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<void>(
      future: ensureEmployeeAppInitialized(),
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) {
          return const Scaffold(body: Center(child: CircularProgressIndicator()));
        }
        return OnefinEmployeeApp(module: module);
      },
    );
  }
}
