import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:flutter/material.dart';

class ConnectivityProvider extends ChangeNotifier {
  ConnectivityProvider(this._connectivity) {
    _subscribe();
  }

  final Connectivity _connectivity;
  bool isOnline = true;

  void _subscribe() {
    _connectivity.onConnectivityChanged.listen((results) {
      isOnline = !results.contains(ConnectivityResult.none);
      notifyListeners();
    });
  }
}
