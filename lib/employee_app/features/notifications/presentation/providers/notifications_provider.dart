import 'package:flutter/material.dart';
import '../../data/repositories/notifications_repository_http.dart';
import '../../domain/entities/notification_entity.dart';

class NotificationsProvider extends ChangeNotifier {
  NotificationsProvider(this._repository);

  final NotificationsRepositoryHttp _repository;

  bool isLoading = false;
  List<NotificationEntity> items = const [];

  int get unreadCount => items.where((n) => !n.isRead).length;

  Future<void> load() async {
    isLoading = true;
    notifyListeners();
    items = await _repository.fetchAll();
    isLoading = false;
    notifyListeners();
  }

  Future<void> markAsRead(String id) async {
    await _repository.markAsRead(id);
    await load();
  }

  Future<void> markAllAsRead() async {
    await _repository.markAllAsRead();
    await load();
  }
}
