import 'package:flutter/material.dart';
import '../storage/local_storage_service.dart';

class AgencyProvider extends ChangeNotifier {
  AgencyProvider(this._storage) {
    _restore();
  }

  final LocalStorageService _storage;

  static const _kAgencyKey = 'selected_agency';

  String? selectedAgency;
  bool _restored = false;
  bool get isRestored => _restored;

  Future<void> _restore() async {
    selectedAgency = await _storage.getString(_kAgencyKey);
    _restored = true;
    notifyListeners();
  }

  Future<void> selectAgency(String agency) async {
    selectedAgency = agency;
    await _storage.setString(_kAgencyKey, agency);
    notifyListeners();
  }
}
