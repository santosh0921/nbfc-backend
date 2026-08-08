import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'core/constants/employee_app_module.dart';
import 'core/di/providers.dart';
import 'core/providers/agency_provider.dart';
import 'core/router/app_router.dart';
import 'core/theme/app_theme.dart';
import 'core/providers/theme_provider.dart';
import 'features/auth/presentation/providers/auth_provider.dart';

export 'core/constants/employee_app_module.dart' show EmployeeAppModule;

class OnefinEmployeeApp extends StatelessWidget {
  const OnefinEmployeeApp({super.key, this.module = EmployeeAppModule.verification});

  final EmployeeAppModule module;

  @override
  Widget build(BuildContext context) {
    return MultiProvider(
      providers: appProviders(module: module),
      child: Consumer3<ThemeProvider, AuthProvider, AgencyProvider>(
        builder: (context, themeProvider, authProvider, agencyProvider, _) {
          return MaterialApp.router(
            title: 'ONEFIN Employee',
            debugShowCheckedModeBanner: false,
            theme: AppTheme.light,
            darkTheme: AppTheme.dark,
            themeMode: themeProvider.themeMode,
            routerConfig: AppRouter.build(authProvider, agencyProvider, module: module),
          );
        },
      ),
    );
  }
}
