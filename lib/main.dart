import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';
import 'package:provider/provider.dart';
import 'core/router/app_router.dart';
import 'core/services/push_notification_service.dart';
import 'core/theme/app_theme.dart';
import 'core/providers/theme_provider.dart';

void main() {
  // Required before any platform-channel call (Firebase.initializeApp()
  // below needs one) — without this, ServicesBinding.instance is still
  // null at this point (runApp() normally creates it implicitly, but
  // that happens too late here since push init runs first), which
  // crashed with "Null check operator used on a null value" deep inside
  // the Firebase plugin on every single app start. That's the actual
  // reason device tokens were never registering: Firebase never
  // finished initializing, so every FirebaseMessaging call after it
  // failed with "[core/no-app]".
  WidgetsFlutterBinding.ensureInitialized();
  // google_fonts defaults to fetching font files from fonts.gstatic.com at
  // runtime if they're not already cached, which hangs/fails with no
  // network — disable runtime fetching so it falls back to the bundled
  // font assets declared in pubspec.yaml immediately instead.
  GoogleFonts.config.allowRuntimeFetching = false;
  // Safe to call before login — it sets up FCM listeners/permission but
  // doesn't register a device token until registerTokenForCurrentUser()
  // runs after a real login (see auth_provider.dart /
  // employee_app/.../auth_provider.dart).
  PushNotificationService.instance.init();
  runApp(const NbfcApp());
}

class NbfcApp extends StatelessWidget {
  const NbfcApp({super.key});

  @override
  Widget build(BuildContext context) {
    return ChangeNotifierProvider(
      create: (_) => ThemeProvider(),
      child: Consumer<ThemeProvider>(
        builder: (context, themeProvider, _) {
          return MaterialApp.router(
            title: 'OneFinance',
            debugShowCheckedModeBanner: false,
            theme: AppTheme.light,
            darkTheme: AppTheme.dark,
            themeMode: themeProvider.themeMode,
            routerConfig: AppRouter.router,
          );
        },
      ),
    );
  }
}
