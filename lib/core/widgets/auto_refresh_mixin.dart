import 'dart:async';

import 'package:flutter/material.dart';

/// Drop-in periodic auto-refresh for any list/dashboard screen that
/// currently only fetches once (on initState) — the exact "have to pull
/// to refresh to see anything new" gap reported across the customer,
/// employee, and admin apps. A screen using this mixin calls
/// [startAutoRefresh] once (typically right after its first load, in
/// initState) and implements [onAutoRefresh] to re-run whatever fetch
/// already backs its FutureBuilder — same call, just on a timer instead
/// of only at construction. Must call [stopAutoRefresh] from dispose().
mixin AutoRefreshMixin<T extends StatefulWidget> on State<T> {
  Timer? _autoRefreshTimer;

  /// How often to poll. 15s balances "feels instant" against not hammering
  /// the backend — override per-screen if a slower/faster cadence fits.
  Duration get autoRefreshInterval => const Duration(seconds: 15);

  /// Re-run this screen's existing fetch. Implementations should just call
  /// whatever setState(_load)-style method already exists.
  Future<void> onAutoRefresh();

  void startAutoRefresh() {
    _autoRefreshTimer?.cancel();
    _autoRefreshTimer = Timer.periodic(autoRefreshInterval, (_) => onAutoRefresh());
  }

  void stopAutoRefresh() {
    _autoRefreshTimer?.cancel();
    _autoRefreshTimer = null;
  }
}
