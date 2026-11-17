# Wyniki turniejów areny

Eksport z lokalnej areny z 16.11.2026: wszystkie turnieje, w których rozegrano partie (17). Eksport tylko odczytywał dane, baza nie została zmieniona.

## Pliki

| Plik | Zawartość |
| --- | --- |
| `index.json` | Lista turniejów: folder, status, liczba partii, wynik SPRT, postęp analizy |
| `engines.json`, `position-sets.json`, `leaderboard.json` | Zarejestrowane silniki, zestawy pozycji, ranking |
| `tournaments/<data>_<id>_<nazwa>/tournament.json` | Konfiguracja, statystyki (Elo, LOS, SPRT) i telemetria |
| `.../games.json` | Partie z API, z numerem pozycji i otwarciem |
| `.../games.pgn` | Rozegrane partie w PGN |
| `.../games.csv` | Jeden wiersz na **zaplanowaną** partię. Nierozegrane mają pusty `result` |
| `.../moves.csv` | Jeden wiersz na ruch: telemetria silnika i ocena Stockfisha |
| `db/chess_arena.dump` | Pełna kopia bazy (`pg_dump --format=custom`) |

Turnieje SPRT planują do 2000 partii i kończą się wcześniej, więc `games.csv` ma wtedy dużo pustych wierszy. Do analiz filtruj po `result`.

## Analiza Stockfisha w chwili eksportu

Analiza nie była ukończona dla części turniejów, np. 1.7.0 vs 1.6.0 (ok. 99%) i 1.6.0 vs 1.5.0 (ok. 96%). Kolumny analizy w `moves.csv` są tam częściowo puste. Postęp per turniej jest w `index.json`. Po zakończeniu analizy eksport warto powtórzyć.

## Przywrócenie bazy

```powershell
docker compose up -d postgres
docker exec -i chess-arena-postgres-1 pg_restore -U chess_arena -d chess_arena --clean --if-exists < results/db/chess_arena.dump
```

`--clean` nadpisuje bieżącą bazę danymi z kopii.
