import 'package:flutter/material.dart';
import '../storage/local_storage_service.dart';

/// The employee app is light-mode only — always report [ThemeMode.light]
/// regardless of the device's system theme, ignoring any dark-mode
/// preference a device may have saved from before this was locked down.
class ThemeProvider extends ChangeNotifier {
  ThemeProvider(this._storage);

  // ignore: unused_field
  final LocalStorageService _storage;

  ThemeMode get themeMode => ThemeMode.light;
}
