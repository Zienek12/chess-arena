# Literatura do pracy inżynierskiej

Zestawienie źródeł dopasowanych do spisu treści pracy i do tego, co faktycznie jest w repozytorium (stan: gałąź `feat/analysis-statistics`, silnik `engine-v1.2.0`). Przy każdym źródle jest krótka notka, **do czego** w pracy się przyda i **który element systemu** uzasadnia.

Oznaczenia: **[A]** artykuł naukowy, **[K]** książka, **[S]** specyfikacja / dokumentacja / standard, **[W]** strona, wiki lub forum, **[?]** dane bibliograficzne do sprawdzenia w katalogu przed wpisaniem do bibliografii.

---

## 2. Teoria gier (szachy)

Szachy w ujęciu teorii gier to gra dwuosobowa, o sumie zerowej, deterministyczna, skończona, z pełną informacją. To wprost prowadzi do twierdzenia Zermelo, minimaksu i dalej do przeszukiwania drzewa gry w rozdziale 3.

| Źródło | Do czego |
| --- | --- |
| **[A]** Zermelo E., *Über eine Anwendung der Mengenlehre auf die Theorie des Schachspiels*, Proc. 5th International Congress of Mathematicians, Cambridge 1912 (wyd. 1913), t. 2, s. 501–504. | Pierwszy wynik matematyczny o szachach: w grze skończonej z pełną informacją jedna ze stron ma strategię wygrywającą albo obie mają strategię remisową. |
| **[A]** Schwalbe U., Walker P., *Zermelo and the Early History of Game Theory*, Games and Economic Behavior 34(1), 2001, s. 123–137. | Współczesne omówienie (i tłumaczenie) pracy Zermelo; wygodne do cytowania zamiast oryginału po niemiecku. |
| **[A]** von Neumann J., *Zur Theorie der Gesellschaftsspiele*, Mathematische Annalen 100, 1928, s. 295–320. | Twierdzenie o minimaksie, podstawa funkcji `negamax`. |
| **[K]** von Neumann J., Morgenstern O., *Theory of Games and Economic Behavior*, Princeton University Press, 1944. | Klasyczna podstawa teorii gier: gry o sumie zerowej, postać ekstensywna (drzewo gry). |
| **[K]** Straffin P.D., *Teoria gier*, tłum. J. Haman, Wydawnictwo Naukowe Scholar, Warszawa 2001 (wyd. 2 popr. 2004). | Polski podręcznik: gry o sumie zerowej, strategie, punkt siodłowy. Dobry do definicji w języku polskim. |
| **[K]** Malawski M., Wieczorek A., Sosnowska H., *Konkurencja i kooperacja. Teoria gier w ekonomii i naukach społecznych*, PWN, Warszawa 1997. **[?]** | Drugi polski podręcznik, formalne definicje gry w postaci ekstensywnej. |
| **[K]** Płatkowski T., *Wstęp do teorii gier*, Uniwersytet Warszawski, 2012 (skrypt). **[?]** | Darmowy polski skrypt akademicki, często w sylabusach. |
| **[A]** Shannon C.E., *Programming a Computer for Playing Chess*, Philosophical Magazine, ser. 7, 41(314), 1950, s. 256–275. | Absolutny klasyk: liczba Shannona (~10¹²⁰ partii), strategie typu A (pełne przeszukiwanie) i B (selektywne), funkcja oceny. Most między rozdziałem 2 a 3. |
| **[A]** Turing A.M., *Digital Computers Applied to Games*, w: Bowden B.V. (red.), *Faster than Thought*, Pitman, London 1953. | Historyczny kontekst: pierwszy „program” szachowy liczony ręcznie. |
| **[A]** Allis L.V., *Searching for Solutions in Games and Artificial Intelligence*, rozprawa doktorska, University of Limburg, Maastricht 1994. | Złożoność przestrzeni stanów i drzewa gry dla szachów i innych gier; tabela porównawcza gier. |
| **[W]** Tromp J., *ChessPositionRanking*, GitHub, 2021, https://github.com/tromp/ChessPositionRanking ; dyskusja: [TalkChess](https://www.talkchess.com/viewtopic.php?p=975229). | Aktualne oszacowanie liczby legalnych pozycji: ok. 4,8 × 10⁴⁴. |
| **[S]** FIDE, *Laws of Chess* (FIDE Handbook, aktualna wersja), https://handbook.fide.com | Formalne reguły, które implementuje `status.rs` i `rules/`: mat, pat, 50 ruchów, trzykrotne powtórzenie, niewystarczający materiał. |
| **[K]** Russell S., Norvig P., *Artificial Intelligence: A Modern Approach*, wyd. 4, Pearson, 2020, rozdz. 5 „Adversarial Search and Games”. Polskie wydanie: *Sztuczna inteligencja. Nowe spojrzenie*, Helion. **[?]** (rok/tom polskiego wydania) | Podręcznikowe ujęcie minimaksu, alfa-beta, funkcji oceny; łatwe do cytowania definicje. |
| **[K]** Mańdziuk J., *Knowledge-Free and Learning-Based Methods in Intelligent Game Playing*, Springer (Studies in Computational Intelligence 276), 2010, DOI 10.1007/978-3-642-11678-0. | Polski autor (PW), przegląd metod AI w grach umysłowych, w tym szachach. |
| **[A]** Campbell M., Hoane A.J., Hsu F., *Deep Blue*, Artificial Intelligence 134(1–2), 2002, s. 57–83. | Historia: maszyna, która pokonała Kasparowa; ilustracja podejścia „przeszukiwanie + ręczna ocena”. |
| **[A]** Silver D. i in., *A general reinforcement learning algorithm that masters chess, shogi, and Go through self-play*, Science 362(6419), 2018, s. 1140–1144. | Kontrast: AlphaZero (MCTS + sieć) vs klasyczny alfa-beta, który rozwija projekt. |
| **[K]** Levy D., Newborn M., *How Computers Play Chess*, Computer Science Press, 1991. | Historia i przegląd technik programowania szachowego. |

---

## 3. Projekt

### 3.1 Założenia projektowe i architektura (schemat blokowy)

System to w praktyce kilka niezależnych usług (frontend, backend-arena, PostgreSQL, kontenery silników, Stockfish jako analizator) połączonych REST, WebSocket, SSE i protokołem UCI. Poniższe pozycje dobrze opisują dokładnie te wzorce.

| Źródło | Do czego / który element systemu |
| --- | --- |
| **[K]** Bass L., Clements P., Kazman R., *Software Architecture in Practice*, wyd. 4, Addison-Wesley, 2021. | Definicja architektury, atrybuty jakościowe (bezpieczeństwo, testowalność, modyfikowalność) — do sekcji „założenia projektowe”. |
| **[K]** Richards M., Ford N., *Fundamentals of Software Architecture*, O'Reilly, 2020 (pol. *Podstawy architektury oprogramowania dla inżynierów*, Helion). **[?]** | Style architektoniczne: warstwowy, oparty na usługach, sterowany zdarzeniami. Uzasadnienie podziału na frontend / backend / silnik. |
| **[W]** Brown S., *The C4 model for visualising software architecture*, https://c4model.com | Gotowa notacja do „schematu blokowego architektury”: diagram kontekstu → kontenerów → komponentów. Polecam narysować diagram kontenerów C4. |
| **[K]** Newman S., *Building Microservices*, wyd. 2, O'Reilly, 2021. | Podział na usługi, komunikacja synchroniczna (REST) i asynchroniczna (zdarzenia). |
| **[K]** Kleppmann M., *Designing Data-Intensive Applications*, O'Reilly, 2017 (pol. *Przetwarzanie danych w dużej skali*, Helion, 2018). | Transakcje, kolejki zadań, idempotencja, dzierżawy (lease) — dokładnie to, co robią `arena/worker.go` i `analysis/worker.go`. |
| **[K]** Hohpe G., Woolf B., *Enterprise Integration Patterns*, Addison-Wesley, 2003. | Wzorce *Competing Consumers* (wielu workerów pobiera partie z jednej kolejki) i *Publish-Subscribe Channel* (`events/broker.go` → `/api/events`). |
| **[K]** Tanenbaum A.S., van Steen M., *Distributed Systems*, wyd. 3, 2017 (pol. *Systemy rozproszone. Zasady i paradygmaty*, WNT, 2006 — wyd. 2). | Ogólne podstawy systemów rozproszonych, awarie, spójność. |
| **[A]** Gray C.G., Cheriton D.R., *Leases: An Efficient Fault-Tolerant Mechanism for Distributed File Cache Consistency*, Proc. 12th ACM SOSP, 1989. | Źródło pojęcia **dzierżawy (lease)**. Arena „wydzierżawia” partię na 5 minut i odnawia ją przy każdym ruchu — to jest dokładnie ten mechanizm. |
| **[S]** PostgreSQL Global Development Group, *PostgreSQL 16 Documentation*: `SELECT … FOR UPDATE SKIP LOCKED` (The Locking Clause), `LISTEN`/`NOTIFY`. | Kolejka zadań w bazie bez dodatkowego brokera; zdarzenia na żywo. |
| **[A]** Fielding R.T., *Architectural Styles and the Design of Network-based Software Architectures*, rozprawa doktorska, University of California, Irvine, 2000. | Definicja REST — API `/api/tournaments`, `/api/engines` itd. |
| **[S]** Fette I., Melnikov A., *RFC 6455: The WebSocket Protocol*, IETF, 2011. | Gra człowiek–silnik (`play/ws.go`, `/ws`). |
| **[S]** WHATWG, *HTML Living Standard*, rozdz. „Server-sent events”. | Aktualizacje na żywo (`/api/events`, `text/event-stream`). Warto w pracy uzasadnić wybór SSE vs WebSocket (jednokierunkowy strumień, automatyczne wznawianie `retry`). |
| **[W]** Wiggins A., *The Twelve-Factor App*, https://12factor.net | Konfiguracja przez zmienne środowiskowe (`ARENA_WORKERS`, `ARENA_ALLOWED_IMAGES`…), bezstanowe procesy. |

### 3.2 Backend (Go) i izolacja silników (Docker)

| Źródło | Do czego |
| --- | --- |
| **[K]** Donovan A.A.A., Kernighan B.W., *The Go Programming Language*, Addison-Wesley, 2015 (pol. *Język Go. Poznaj i programuj*, Helion, 2016). | Gorutyny, kanały, `context` — model współbieżności workerów i brokera zdarzeń. |
| **[A]** Merkel D., *Docker: Lightweight Linux Containers for Consistent Development and Deployment*, Linux Journal 2014(239), 2014. | Podstawowe źródło o Dockerze. |
| **[A]** Felter W., Ferreira A., Rajamony R., Rubio J., *An Updated Performance Comparison of Virtual Machines and Linux Containers*, IEEE ISPASS, 2015. | Uzasadnienie: kontenery dają izolację przy niewielkim narzucie (ważne przy pomiarze czasu ruchu). |
| **[A]** Combe T., Martin A., Di Pietro R., *To Docker or Not to Docker: A Security Perspective*, IEEE Cloud Computing 3(5), 2016, s. 54–62. | Bezpieczeństwo: dlaczego `--network none`, limity zasobów, allowlista obrazów (`sandbox/policy.go`), oraz ryzyko montowania `docker.sock`. |
| **[S]** Kahlen S.-M., *Description of the Universal Chess Interface (UCI)*, 2004, http://wbec-ridderkerk.nl/html/UCIProtocol.html | Protokół między areną a silnikiem (`uci/client.go`, `engines/src/uci.rs`). Kluczowe źródło. |
| **[S]** Edwards S.J., *Portable Game Notation Specification and Implementation Guide*, 1994. | Formaty PGN (zapis partii) i FEN (pozycje startowe, import zestawów pozycji). |
| **[K]** Humble J., Farley D., *Continuous Delivery*, Addison-Wesley, 2010. | CI/CD: workflowy GitHub Actions (`ci.yml`, `publish-engine.yml`), budowa i publikacja obrazów silnika. |
| **[W]** Preston-Werner T., *Semantic Versioning 2.0.0*, https://semver.org | Wersjonowanie silnika (`engine-vX.Y.Z`). |

### 3.3 Frontend (Next.js)

| Źródło | Do czego |
| --- | --- |
| **[S]** Vercel, *Next.js Documentation*, https://nextjs.org/docs ; Meta, *React Documentation*, https://react.dev | Routing stron (`pages/`), renderowanie. |
| **[S]** Nottingham M., *RFC 5861: HTTP Cache-Control Extensions for Stale Content*, IETF, 2010. | Strategia *stale-while-revalidate*, od której pochodzi nazwa i idea biblioteki SWR używanej we froncie. |
| **[W]** chess.js, react-chessboard — dokumentacja na GitHub/npm. | Walidacja ruchów i szachownica po stronie przeglądarki. |

### 3.4 Systemy pokrewne (do przeglądu „state of the art” / porównania)

Bardzo przydatne, bo pokazują, że architektura projektu wpisuje się w praktykę społeczności szachowej:

| System | Analogia w chess-arena |
| --- | --- |
| **[W]** **Fishtest** — rozproszone testowanie Stockfisha, https://github.com/official-stockfish/fishtest | SPRT jako kryterium przyjęcia zmiany w silniku (`scripts/regress.ps1`). |
| **[W]** **OpenBench** (A. Grant), https://github.com/AndyGrant/OpenBench ; [CPW](https://chessprogramming.org/OpenBench) | Najbliższy odpowiednik całego projektu: serwer + klienci-workerzy, testy SPRT między wersjami silnika. |
| **[W]** **cutechess-cli**, https://github.com/cutechess/cutechess ; **fastchess**, https://github.com/Disservin/fastchess | Narzędzia do meczów silnik–silnik przez UCI (to, co robi `arena/match.go`). |
| **[W]** **Lichess fishnet**, https://github.com/lichess-org/fishnet ; **lila**, https://github.com/lichess-org/lila | Rozproszona analiza partii Stockfishem — odpowiednik `analysis/worker.go`. |

### 3.5 Silnik szachowy — kodowanie i reprezentacja

| Źródło | Do czego / plik |
| --- | --- |
| **[A]** Adelson-Velsky G.M., Arlazarov V.L., Bitman A.R., Zhivotovsky A.A., Uskov A.V., *Programming a Computer to Play Chess*, Russian Mathematical Surveys 25(2), 1970, s. 221–262. | Wczesne użycie bitboardów (program Kaissa). |
| **[A]** Slate D.J., Atkin L.R., *CHESS 4.5 – The Northwestern University Chess Program*, w: Frey P.W. (red.), *Chess Skill in Man and Machine*, Springer, 1977, s. 82–118. | Bitboardy i iteracyjne pogłębianie — dwa filary `board.rs` i `search/mod.rs`. |
| **[A]** Hyatt R.M., *Rotated Bitmaps, a New Twist on an Old Idea*, ICCA Journal 22(4), 1999, s. 213–222. | Generowanie ataków figur liniowych; tło dla planowanej optymalizacji `attacks.rs`. |
| **[A]** Kannan P., *Magic Move-Bitboard Generation in Computer Chess*, 2007 (praca udostępniona przez autora; opis: [CPW](https://chessprogramming.org/Pradu_Kannan)). | *Magic bitboards* — następny krok po obecnym ray-tracingu. |
| **[A]** Zobrist A.L., *A New Hashing Method with Application for Game Playing*, Tech. Rep. 88, University of Wisconsin, 1970 (przedruk: ICCA Journal 13(2), 1990). | Haszowanie pozycji (`hash.rs`), wykrywanie powtórzeń, przyszła tablica transpozycji. |
| **[A]** Steele G.L., Lea D., Flood C.H., *Fast Splittable Pseudorandom Number Generators*, OOPSLA 2014. | Źródło funkcji **SplitMix64**, z której `hash.rs` generuje klucze Zobrista. |
| **[K]** Warren H.S., *Hacker's Delight*, wyd. 2, Addison-Wesley, 2012. | Operacje bitowe: `popcount`, `trailing_zeros`, `pop_lsb`. |
| **[K]** Klabnik S., Nichols C., *The Rust Programming Language*, wyd. 2, No Starch Press, 2023 (też online: https://doc.rust-lang.org/book). | Język silnika; własność, typy `Copy`, wątki i `Arc<AtomicBool>` w `uci.rs`. |
| **[W]** Chess Programming Wiki: *Bitboards*, *Square Mapping Considerations* (LERF), *Perft Results*, *Make Move / Unmake Move*, https://www.chessprogramming.org | Konwencja a1=0…h8=63; wartości referencyjne perft z testów. |

### 3.6 Algorytmy (podrozdziały)

Kolejność odpowiada temu, co jest w v1.2.0, a potem temu, co jest w planie („Known weaknesses” w `engine-internals-v.1.2.0.md`).

**Już zaimplementowane**

| Algorytm | Źródło |
| --- | --- |
| Minimax / negamax, alfa-beta | **[A]** Knuth D.E., Moore R.W., *An Analysis of Alpha-Beta Pruning*, Artificial Intelligence 6(4), 1975, s. 293–326 — formalna analiza, sformułowanie negamax, złożoność ~b^(d/2) przy idealnym porządku. **[A]** Pearl J., *The Solution for the Branching Factor of the Alpha-Beta Pruning Algorithm and Its Optimality*, Communications of the ACM 25(8), 1982, s. 559–564. **[A]** Marsland T.A., *A Review of Game-Tree Pruning*, ICCA Journal 9(1), 1986, s. 3–19. |
| Iteracyjne pogłębianie | **[A]** Korf R.E., *Depth-First Iterative-Deepening: An Optimal Admissible Tree Search*, Artificial Intelligence 27(1), 1985, s. 97–109; Slate i Atkin (1977, wyżej). |
| Porządkowanie ruchów (PV, MVV-LVA) | **[W]** CPW: *Move Ordering*, *MVV-LVA*; Marsland 1986. |
| Funkcja oceny: materiał + tablice pozycyjne | **[W]** Michniewski T., *Simplified Evaluation Function*, CPW, https://www.chessprogramming.org/Simplified_Evaluation_Function — wartości i tablice są wzięte wprost stąd (polski autor). |
| Tapered eval (faza gry) | **[W]** CPW: *Tapered Eval* (podejście z programu Fruit, F. Letouzey). |
| Zarządzanie czasem | **[A]** Hyatt R.M., *Using Time Wisely*, ICCA Journal 7(1), 1984, s. 4–9; **[W]** CPW: *Time Management*. |
| Oceny matowe, wykrywanie remisów | **[W]** CPW: *Checkmate*, *Repetitions*; reguły FIDE. |

**Planowane (do kolejnych wersji i turniejów)**

| Algorytm | Źródło |
| --- | --- |
| Quiescence search | **[A]** Beal D.F., *A Generalised Quiescence Search Algorithm*, Artificial Intelligence 43(1), 1990, s. 85–98; Shannon 1950 (problem „spokojnych” pozycji); **[W]** CPW: *Horizon Effect*. |
| Tablica transpozycji | **[A]** Breuker D.M., Uiterwijk J.W.H.M., van den Herik H.J., *Replacement Schemes for Transposition Tables*, ICCA Journal 17(4), 1994, s. 183–193; Zobrist 1970. |
| Killer moves | **[A]** Akl S.G., Newborn M.M., *The Principal Continuation and the Killer Heuristic*, Proc. ACM Annual Conference, 1977, s. 466–473. |
| History heuristic | **[A]** Schaeffer J., *The History Heuristic and Alpha-Beta Search Enhancements in Practice*, IEEE Transactions on Pattern Analysis and Machine Intelligence 11(11), 1989, s. 1203–1212 — mierzy też wpływ TT, killerów i aspiracji; świetny wzór do rozdziału z testami. |
| PVS / NegaScout | **[A]** Marsland T.A., Campbell M.S., *Parallel Search of Strongly Ordered Game Trees*, ACM Computing Surveys 14(4), 1982, s. 533–551; **[A]** Reinefeld A., *An Improvement to the Scout Tree Search Algorithm*, ICCA Journal 6(4), 1983, s. 4–14. |
| Okna aspiracyjne / MTD(f) | **[A]** Plaat A., Schaeffer J., Pijls W., de Bruin A., *Best-First Fixed-Depth Minimax Algorithms*, Artificial Intelligence 87(1–2), 1996, s. 255–293. |
| Null-move pruning | **[A]** Beal D.F., *Experiments with the Null Move*, w: *Advances in Computer Chess 5*, 1989; **[A]** Donninger C., *Null Move and Deep Search*, ICCA Journal 16(3), 1993, s. 137–143; **[A]** Heinz E.A., *Adaptive Null-Move Pruning*, ICCA Journal 22(3), 1999, s. 123–132. |
| Late Move Reductions | **[W]** Romstad T., *An Introduction to Late Move Reductions*, 2007 (opis na CPW: *Late Move Reductions*). |
| Selektywne przeszukiwanie ogólnie | **[K]** Heinz E.A., *Scalable Search in Computer Chess*, Vieweg, 2000. |
| Strojenie oceny | **[W]** Österlund P., *Texel's Tuning Method* (TalkChess 2014; CPW: *Texel's Tuning Method*). |
| Ocena siecią (opcjonalnie, jako perspektywa w podsumowaniu) | **[A]** Nasu Y., *Efficiently Updatable Neural-Network-based Evaluation Functions for Computer Shogi*, Ziosoft Computer Shogi Club, 2018 (po japońsku; nieoficjalne tłumaczenie ang.: D. Klein, https://github.com/asdfjkl/nnue). |

**Ogólne źródła praktyczne (forum / wiki — dopuszczalne wg promotora)**

- **[W]** Chess Programming Wiki, https://www.chessprogramming.org — główna encyklopedia dziedziny; ma bibliografię do każdego tematu (można z niej „przeskakiwać” do artykułów ICCA Journal).
- **[W]** TalkChess / Computer Chess Club, https://talkchess.com — forum twórców silników (dyskusje o SPRT, LMR, tuningu).
- **[W]** Laramée F.D., *Chess Programming*, cz. I–VI, GameDev.net, 2000 — przystępny kurs, dobry do wstępu do rozdziału.
- **[W]** Stockfish, kod źródłowy, https://github.com/official-stockfish/Stockfish — punkt odniesienia i silnik analizujący w arenie.

### 3.7 Testy (turnieje między wersjami) — statystyka

| Element w kodzie (`stats/`, `analysis/`) | Źródło |
| --- | --- |
| Model Elo, logistyczna różnica Elo | **[K]** Elo A.E., *The Rating of Chessplayers, Past and Present*, Arco, New York 1978. |
| Ranking (Bradley–Terry, iteracje MM) | **[A]** Bradley R.A., Terry M.E., *Rank Analysis of Incomplete Block Designs: I. The Method of Paired Comparisons*, Biometrika 39(3/4), 1952, s. 324–345. **[A]** Hunter D.R., *MM Algorithms for Generalized Bradley–Terry Models*, Annals of Statistics 32(1), 2004, s. 384–406 — to dokładnie algorytm z `stats/ratings.go`. |
| Alternatywy rankingowe (do dyskusji) | **[A]** Coulom R., *Whole-History Rating: A Bayesian Rating System for Players of Time-Varying Strength*, Computers and Games 2008, LNCS 5131, s. 113–124; BayesElo, https://www.remi-coulom.fr/Bayesian-Elo/ . **[A]** Glickman M.E., *Parameter Estimation in Large Dynamic Paired Comparison Experiments*, Journal of the Royal Statistical Society C 48(3), 1999, s. 377–394 (Glicko). |
| SPRT | **[A]** Wald A., *Sequential Tests of Statistical Hypotheses*, Annals of Mathematical Statistics 16(2), 1945, s. 117–186; **[K]** Wald A., *Sequential Analysis*, Wiley, 1947. |
| GSPRT w testowaniu silników (wzór LLR z `stats.go`) | **[W]** Van den Bergh M., *A Practical Introduction to the GSPRT*, 2017; *The Generalized Maximum Likelihood Ratio for the Expectation Value of a Distribution*, 2019 (dokumenty Fishtest; bibliografia na [CPW: SPRT](https://chessprogramming.org/Sequential_Probability_Ratio_Test)). |
| LOS, przedział błędu Elo | **[W]** CPW: *Match Statistics*, *Elo Rating*; Fishtest wiki. |
| Jakość ruchów względem Stockfisha | **[A]** Guid M., Bratko I., *Computer Analysis of World Chess Champions*, ICGA Journal 29(2), 2006, s. 65–73 — ta sama idea co `cp_loss`. **[A]** Regan K.W., Haworth G.McC., *Intrinsic Chess Ratings*, Proc. AAAI 2011. **[W]** Lichess, *Accuracy metric*, https://lichess.org/page/accuracy — wzory win% i accuracy użyte w `analysis/compute.go`. |
| Poprawność generatora (perft) jako test jednostkowy | **[W]** CPW: *Perft*, *Perft Results*. |

---

## Uwagi do treści pracy, które wyszły przy przeglądzie repozytorium

1. **Niespójności w dokumentacji** (warto poprawić przed pisaniem rozdziału 3, żeby nie przepisać ich do pracy):
   - `docs/README.md` wymienia **Jenkins** (CI/CD) i **WebAssembly** we froncie — w repozytorium jest GitHub Actions, a walidację ruchów we froncie robi `chess.js` (bez WASM).
   - Główny `README.md` w ostatnim punkcie mówi, że silnik „does not search for the best move yet” oraz że wybiera losowy ruch — od v1.2.0 to nieaktualne (jest alfa-beta z iteracyjnym pogłębianiem).
2. **Statystyka a pary partii.** Turnieje grają każdą pozycję dwa razy z zamianą kolorów, a `elo_error` i LLR liczone są z modelu trójmianowego (W/D/L). Fishtest i fastchess przy takich parach używają modelu **pentanomialnego** (wynik pary: 0, ½, 1, 1½, 2), który daje węższe przedziały i szybsze decyzje SPRT (Van den Bergh). To dobry temat albo na usprawnienie, albo na akapit „dyskusja/ograniczenia” w rozdziale z testami.
3. **Rozdział „Testy”** ma gotową oś: v1.0 (losowy ruch) → v1.2.0 (alfa-beta + PST) → kolejne wersje (quiescence, TT, killers/history…). Dla każdej wersji: turniej SPRT z poprzednią, Elo ± błąd, LOS, średnia głębokość/NPS z telemetrii i accuracy/CPL ze Stockfisha. Wzorem prezentacji wyników może być tabela z pracy Schaeffera (1989), który mierzył wkład każdej techniki osobno.
4. **Weryfikacja przed wpisaniem do bibliografii:** pozycje oznaczone **[?]** sprawdź w katalogu BG PWr / NUKAT (rok i wydawca polskich wydań). Artykuły z ICCA Journal są dostępne przez IOS Press (ICGA Journal) lub jako przedruki na stronach autorów.
