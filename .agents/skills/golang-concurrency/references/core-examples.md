# Примеры основных приёмов

Команды используют уже существующий защищённый endpoint pprof. Не открывай
диагностический порт наружу и не вводи новую инфраструктуру ради примера.
Доступность профиля проверяй в версии Go проекта; отсутствие профиля не мешает
анализу обычного goroutine dump, счётчика и race detector.

## Профиль утечек горутин

```bash
curl http://localhost:6060/debug/pprof/goroutineleak?debug=2
go tool pprof http://localhost:6060/debug/pprof/goroutineleak
```
