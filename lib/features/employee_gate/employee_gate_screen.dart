import 'package:flutter/material.dart';
import '../../employee_app/app.dart';
import '../../employee_app/bootstrap.dart';

/// NBFC-side entry point into the Employee role, reached directly from
/// "Login as Employee" on the customer login screen — no role-select or
/// agency-select step first. Runs the one-time Hive/ServiceLocator init the
/// ported onefin_employee app needs, then mounts its own [OnefinEmployeeApp]
/// widget, which owns its own internal `MaterialApp.router`, navigation
/// stack, and auth state from that point on, independent of NBFC's
/// customer-side router.
///
/// [OnefinEmployeeApp] always starts at its login screen regardless of
/// which module is passed here — the module only matters as a routing
/// default before anyone is authenticated. Once the employee logs in, the
/// dashboard they land on (Verification vs Recovery) is derived from their
/// actual role, not this value — see
/// `employee_app/core/router/app_router.dart`'s `_homePathFor`.
class EmployeeGateScreen extends StatelessWidget {
  const EmployeeGateScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<void>(
      future: ensureEmployeeAppInitialized(),
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) {
          return const Scaffold(body: Center(child: CircularProgressIndicator()));
        }
        return const OnefinEmployeeApp();
      },
    );
  }
}
