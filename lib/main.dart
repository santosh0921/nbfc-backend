import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';
import 'package:provider/provider.dart';
import 'core/router/app_router.dart';
import 'core/theme/app_theme.dart';
import 'core/providers/theme_provider.dart';

void main() {
  // google_fonts defaults to fetching font files from fonts.gstatic.com at
  // runtime if they're not already cached, which hangs/fails with no
  // network — disable runtime fetching so it falls back to the bundled
  // font assets declared in pubspec.yaml immediately instead.
  GoogleFonts.config.allowRuntimeFetching = false;
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
            title: 'NBFC Premium',
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
