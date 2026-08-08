import '../../domain/entities/collection_entity.dart';

class CollectionsRepositoryMock {
  static final _items = <CollectionEntity>[
    CollectionEntity(
      id: 'col-1',
      customerName: 'Deepak Kumar',
      loanAccountNumber: 'OF-LN-77231',
      emiAmount: 6200,
      dueDate: DateTime.now(),
      status: CollectionStatus.pending,
    ),
    CollectionEntity(
      id: 'col-2',
      customerName: 'Anjali Desai',
      loanAccountNumber: 'OF-LN-77345',
      emiAmount: 4500,
      dueDate: DateTime.now(),
      status: CollectionStatus.collected,
    ),
    CollectionEntity(
      id: 'col-3',
      customerName: 'Vikram Singh',
      loanAccountNumber: 'OF-LN-77210',
      emiAmount: 9800,
      dueDate: DateTime.now().subtract(const Duration(days: 3)),
      status: CollectionStatus.overdue,
    ),
    CollectionEntity(
      id: 'col-4',
      customerName: 'Kavita Rao',
      loanAccountNumber: 'OF-LN-77400',
      emiAmount: 3100,
      dueDate: DateTime.now(),
      status: CollectionStatus.pending,
    ),
  ];

  Future<List<CollectionEntity>> fetchCollections() async {
    await Future.delayed(const Duration(milliseconds: 400));
    return List.unmodifiable(_items);
  }

  Future<void> confirmPayment(String id) async {
    await Future.delayed(const Duration(milliseconds: 500));
    final index = _items.indexWhere((c) => c.id == id);
    if (index != -1) {
      _items[index] = CollectionEntity(
        id: _items[index].id,
        customerName: _items[index].customerName,
        loanAccountNumber: _items[index].loanAccountNumber,
        emiAmount: _items[index].emiAmount,
        dueDate: _items[index].dueDate,
        status: CollectionStatus.collected,
      );
    }
  }
}
