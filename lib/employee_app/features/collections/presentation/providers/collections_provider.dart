import 'package:flutter/material.dart';
import '../../data/repositories/collections_repository_mock.dart';
import '../../domain/entities/collection_entity.dart';

class CollectionsProvider extends ChangeNotifier {
  CollectionsProvider(this._repository);

  final CollectionsRepositoryMock _repository;

  bool isLoading = false;
  List<CollectionEntity> items = const [];

  double get todaysTotal =>
      items.where((c) => c.status == CollectionStatus.collected).fold(0, (sum, c) => sum + c.emiAmount);

  List<CollectionEntity> get pending => items.where((c) => c.status != CollectionStatus.collected).toList();

  Future<void> load() async {
    isLoading = true;
    notifyListeners();
    items = await _repository.fetchCollections();
    isLoading = false;
    notifyListeners();
  }

  Future<void> confirmPayment(String id) async {
    await _repository.confirmPayment(id);
    await load();
  }
}
