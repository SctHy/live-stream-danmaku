# Maku Evaluation

darwin/arm64 · go1.27.4 · CPUs 10 · profile full · repeats 5 (+ warmup) · 2026-11-25T20:22:59.428944Z · git_sha 46d00557c2410193666931552837a565d8351ce8 · dirty false

## Scorecard

| Experiment / load | Metric | Baseline → treatment | Medians | Change (%) | Δ 95% CI | Trade-off | Gate |
| --- | --- | --- | --- | --- | --- | --- | --- |
| fanout_sweep low | frames | legacy → adaptive_batch | 6400 → 6400 | 0 | 0 … 0 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | hot_p50_us | legacy → adaptive_batch | 436 → 442 | 1.3761467889908285 | -1084 … 1121 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | hot_p99_us | legacy → adaptive_batch | 2057 → 2313 | 12.445308701993184 | -1713 … 1604 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | process_cpu_seconds_total | legacy → adaptive_batch | 0.157549 → 0.170231 | 8.049559184761556 | -0.22956 … 0.23633600000000002 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | frames | legacy → mixed_25pct_legacy | 6400 → 6400 | 0 | 0 … 0 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | hot_p50_us | legacy → mixed_25pct_legacy | 436 → 408 | -6.422018348623848 | -1111 … 1101 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | hot_p99_us | legacy → mixed_25pct_legacy | 2057 → 982 | -52.26057365094798 | -2600 … 1428 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | process_cpu_seconds_total | legacy → mixed_25pct_legacy | 0.157549 → 0.14366700000000002 | -8.811226983351194 | -0.24774199999999996 … 0.245532 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | frames | legacy → adaptive_batch | 76800 → 76800 | 0 | 0 … 0 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | hot_p50_us | legacy → adaptive_batch | 1085 → 1146 | 5.622119815668203 | -111 … 751 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | hot_p99_us | legacy → adaptive_batch | 2889 → 2847 | -1.4537902388369717 | -736 … 3868 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | process_cpu_seconds_total | legacy → adaptive_batch | 1.273327 → 1.251758 | -1.693908948761802 | -0.08411999999999997 … 0.630304 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | frames | legacy → mixed_25pct_legacy | 76800 → 76800 | 0 | 0 … 0 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | hot_p50_us | legacy → mixed_25pct_legacy | 1085 → 1236 | 13.917050691244249 | -376 … 841 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | hot_p99_us | legacy → mixed_25pct_legacy | 2889 → 2609 | -9.691934925579782 | -974 … 4255 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | process_cpu_seconds_total | legacy → mixed_25pct_legacy | 1.273327 → 1.3127449999999998 | 3.095669847572524 | -0.25442900000000024 … 0.6912909999999999 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | frames | legacy → adaptive_batch | 313600 → 99008 | -68.42857142857143 | -215712 … -212352 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | hot_p50_us | legacy → adaptive_batch | 610 → 4010 | 557.3770491803278 | 3329 … 3582 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | hot_p99_us | legacy → adaptive_batch | 5238 → 8518 | 62.61932035127911 | -1602 … 5017 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | process_cpu_seconds_total | legacy → adaptive_batch | 2.5383 → 1.331456 | -47.54536500807627 | -1.3267930000000001 … -1.1112640000000003 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | frames | legacy → mixed_25pct_legacy | 313600 → 144928 | -53.78571428571428 | -169680 … -167832 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | hot_p50_us | legacy → mixed_25pct_legacy | 610 → 3188 | 422.6229508196721 | 2507 … 2635 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | hot_p99_us | legacy → mixed_25pct_legacy | 5238 → 7526 | 43.68079419625812 | -2594 … 11557 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | process_cpu_seconds_total | legacy → mixed_25pct_legacy | 2.5383 → 1.657621 | -34.695623054800464 | -1.095566 … -0.7750220000000001 | Batching can increase hot-room latency while reducing frames. | PASS |
| isolation_stalled room_shards | cold_p99_us | single_shard → default_shards | 2760 → 2151 | -22.065217391304348 | -1935 … -43 |  | PASS |
| isolation_stalled room_shards | danmaku_dropped_messages_total{reason="client_full"} | single_shard → default_shards | 0 → 0 | — | 0 … 0 |  | PASS |
| isolation_stalled room_shards | danmaku_kicked_slow_clients_total | single_shard → default_shards | 0 → 0 | — | 0 … 0 |  | PASS |
| isolation_stalled room_shards | rss_peak_bytes | single_shard → default_shards | 46350336 → 47087616 | 1.590668080593849 | -1163264 … 1572864 |  | PASS |
| isolation_stalled slow_kick | cold_p99_us | kick_disabled → kick_enabled | 2593 → 2385 | -8.021596606247595 | -3322 … 5491 |  | PASS |
| isolation_stalled slow_kick | danmaku_dropped_messages_total{reason="client_full"} | kick_disabled → kick_enabled | 11992 → 32 | -99.73315543695797 | -11960 … -11296 |  | PASS |
| isolation_stalled slow_kick | danmaku_kicked_slow_clients_total | kick_disabled → kick_enabled | 0 → 4 | — | 4 … 4 |  | PASS |
| isolation_stalled slow_kick | rss_peak_bytes | kick_disabled → kick_enabled | 60850176 → 59899904 | -1.561658589122239 | -2310144 … 245760 |  | PASS |
| migration_strategies | connect_peak_qps | crash → fixed | 330 → 390 | 18.181818181818187 | 10 … 160 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| migration_strategies | drain_ms | crash → fixed | 452 → 404.901 | -10.420132743362831 | -51.09899999999999 … -43.09899999999999 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| migration_strategies | recent_peak_qps | crash → fixed | 330 → 480 | 45.45454545454546 | 100 … 160 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| migration_strategies | connect_peak_qps | crash → bounded | 330 → 80 | -75.75757575757575 | -290 … -240 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| migration_strategies | drain_ms | crash → bounded | 452 → 1905.346 | 321.5367256637168 | 1449.346 … 1457.346 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| migration_strategies | recent_peak_qps | crash → bounded | 330 → 80 | -75.75757575757575 | -290 … -240 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| history_pagination page_1 | p50_us | offset_baseline → cursor_before_id_prod | 383.042 → 387.416 | 1.141911330872336 | -15.375 … 20.83299999999997 | Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps. | PASS |
| history_pagination page_200 | p50_us | offset_baseline → cursor_before_id_prod | 9304.458 → 416.583 | -95.52275909031994 | -8949.832999999999 … -8632.667 | Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps. | PASS |
| history_pagination page_1000 | p50_us | offset_baseline → cursor_before_id_prod | 37296.708 → 408.458 | -98.90484168200582 | -37681.751000000004 … -36535.959 | Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps. | PASS |
| history_pagination page_2000 | p50_us | offset_baseline → cursor_before_id_prod | 89780.959 → 496 | -99.44754432841378 | -92143.125 … -88877.458 | Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps. | PASS |
| history_pagination page_3600 | p50_us | offset_baseline → cursor_before_id_prod | 161060.5 → 455.417 | -99.71723855321447 | -161508.41600000003 … -159422.124 | Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps. | PASS |
| pg_archive_write batch_128 | rows_per_second | row_by_row_tx → multivalue_insert | 4820.906235317528 → 69944.27239276672 | 1350.8532001796932 | 61641.642175769084 … 69387.97935923794 | Production UNNEST binds nine arrays per batch. The comparison inserts supply five fields per row. | PASS |
| pg_archive_write batch_128 | rows_per_second | row_by_row_tx → unnest_prod | 4820.906235317528 → 75254.88550454212 | 1461.0111840224472 | 64772.331511558776 … 72250.5977135042 | Production UNNEST binds nine arrays per batch. The comparison inserts supply five fields per row. | PASS |
| redis_lua_atomicity 0s | p50_us | multi_cmd_baseline → lua_evalsha_prod | 715.875 → 96.292 | -86.54904836738257 | -624.7909999999999 … -615.334 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| redis_lua_atomicity 0s | qps | multi_cmd_baseline → lua_evalsha_prod | 1393.5582768651907 → 10193.37650703975 | 631.4639564245387 | 8435.60120219786 … 8983.338931251368 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| redis_lua_atomicity 0s | race_duplicate_rate | multi_cmd_baseline → lua_evalsha_prod | 0.21875 → 0 | -100 | -0.21875 … -0.21875 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| redis_lua_atomicity 2ms | p50_us | multi_cmd_baseline → lua_evalsha_prod | 25138.583 → 2881.875 | -88.53604835244691 | -23135.292 … -21777.459 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| redis_lua_atomicity 2ms | qps | multi_cmd_baseline → lua_evalsha_prod | 39.77963276835316 → 344.1684253443756 | 765.1875379256394 | 288.55804283073996 … 353.17971460476275 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| redis_lua_atomicity 2ms | race_duplicate_rate | multi_cmd_baseline → lua_evalsha_prod | 0.21875 → 0 | -100 | -0.21875 … -0.21875 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| sensitive_filter_scaling words_100 | miss_us | naive_contains → merged_regexp | 0.7 → 99.04 | 14048.57142857143 | 96.72208 … 99.91833 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling words_100 | miss_us | naive_contains → ac_prod | 0.7 → 1.49125 | 113.0357142857143 | 0.6558300000000001 … 0.8674999999999999 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling words_1000 | miss_us | naive_contains → merged_regexp | 6.717910000000001 → 1540.4379199999998 | 22830.31493425782 | 1468.50542 … 1603.6170699999998 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling words_1000 | miss_us | naive_contains → ac_prod | 6.717910000000001 → 1.49208 | -77.78952084800184 | -5.388750000000001 … -5.023340000000001 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling words_5000 | miss_us | naive_contains → ac_prod | 34.645 → 1.49166 | -95.69444364266127 | -33.30292 … -32.775839999999995 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling words_20000 | miss_us | naive_contains → ac_prod | 138.57875 → 1.46 | -98.94644741708234 | -137.28083 … -136.97876 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling reload_5000 | reload_p99_us | rwmutex_rebuild_baseline → atomic_pointer_prod | 25359.292 → 10 | -99.96056672244636 | -27883.583 … -22828.458 | Atomic reload temporarily retains old and new tries. | PASS |
| live_source_singleflight burst_120 | cold_upstream_calls | direct_no_coalesce → prod_singleflight_cache | 120 → 1 | -99.16666666666667 | -119 … -119 | Live results may remain cached for 15 seconds. | PASS |
| live_source_singleflight burst_120 | warm_upstream_calls | direct_no_coalesce → prod_singleflight_cache | 120 → 0 | -100 | -120 … -120 | Live results may remain cached for 15 seconds. | PASS |

## fanout_sweep low · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| legacy | achieved_rate | 99.94852860668666 | 99.944847085226 … 99.95619209973226 | 5 |
| legacy | affected | 0 | 0 … 0 | 5 |
| legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| legacy | cold_gaps | 0 | 0 … 0 | 5 |
| legacy | cold_missing | 0 | 0 … 0 | 5 |
| legacy | cold_p50_us | 413 | 367 … 1467 | 5 |
| legacy | cold_p99_us | 1938 | 943 … 3191 | 5 |
| legacy | cold_rate | 100 | 100 … 100 | 5 |
| legacy | cold_received | 200 | 200 … 200 | 5 |
| legacy | cold_samples | 200 | 200 … 200 | 5 |
| legacy | cold_unique | 200 | 200 … 200 | 5 |
| legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_count | 400 | 400 … 400 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_sum | 0.0004163860000000001 | 0.00028669800000000016 … 0.001985218000000001 | 5 |
| legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="single"} | 10400 | 10400 … 10400 | 5 |
| legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| legacy | duplicates | 0 | 0 … 0 | 5 |
| legacy | expected | 6400 | 6400 … 6400 | 5 |
| legacy | frames | 6400 | 6400 … 6400 | 5 |
| legacy | go_gc_duration_seconds_sum | 0.000329707 | 0.00021341700000000003 … 0.00042062500000000005 | 5 |
| legacy | go_goroutines | 1 | 1 … 2 | 5 |
| legacy | go_memstats_alloc_bytes_total | 12797288 | 12442768 … 14972616 | 5 |
| legacy | goroutines_peak | 178 | 178 … 178 | 5 |
| legacy | hot_p50_us | 436 | 377 … 1511 | 5 |
| legacy | hot_p99_us | 2057 | 1033 … 3582 | 5 |
| legacy | hot_samples | 1600 | 1600 … 1600 | 5 |
| legacy | missing | 0 | 0 … 0 | 5 |
| legacy | payload_bytes | 2194112 | 2193664 … 2194624 | 5 |
| legacy | process_cpu_seconds_total | 0.157549 | 0.144014 … 0.390642 | 5 |
| legacy | process_resident_memory_bytes | 11091968 | 10076160 … 11517952 | 5 |
| legacy | received | 6400 | 6400 … 6400 | 5 |
| legacy | rss_peak_bytes | 36356096 | 35307520 … 36831232 | 5 |
| legacy | seconds | 2 | 2 … 2 | 5 |
| legacy | target_rate | 100 | 100 … 100 | 5 |
| legacy | viewers | 32 | 32 … 32 | 5 |
| legacy | worst_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | achieved_rate | 99.94866596536552 | 99.94629760496814 … 99.94870967047889 | 5 |
| adaptive_batch | affected | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_p50_us | 411 | 375 … 1421 | 5 |
| adaptive_batch | cold_p99_us | 2249 | 664 … 3651 | 5 |
| adaptive_batch | cold_rate | 100 | 100 … 100 | 5 |
| adaptive_batch | cold_received | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_samples | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_unique | 200 | 200 … 200 | 5 |
| adaptive_batch | cursor_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_count | 400 | 400 … 400 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_sum | 0.0004519690000000001 | 0.00037187399999999963 … 0.002295121 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="single"} | 10400 | 10400 … 10400 | 5 |
| adaptive_batch | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| adaptive_batch | duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | expected | 6400 | 6400 … 6400 | 5 |
| adaptive_batch | frames | 6400 | 6400 … 6400 | 5 |
| adaptive_batch | go_gc_duration_seconds_sum | 0.000339791 | 0.000296084 … 0.0005295839999999999 | 5 |
| adaptive_batch | go_goroutines | 1 | 1 … 3 | 5 |
| adaptive_batch | go_memstats_alloc_bytes_total | 14360224 | 11278968 … 15000376 | 5 |
| adaptive_batch | goroutines_peak | 179 | 178 … 179 | 5 |
| adaptive_batch | hot_p50_us | 442 | 399 … 1510 | 5 |
| adaptive_batch | hot_p99_us | 2313 | 614 … 2999 | 5 |
| adaptive_batch | hot_samples | 1600 | 1600 … 1600 | 5 |
| adaptive_batch | missing | 0 | 0 … 0 | 5 |
| adaptive_batch | payload_bytes | 2194656 | 2193952 … 2195136 | 5 |
| adaptive_batch | process_cpu_seconds_total | 0.170231 | 0.154495 … 0.38968400000000003 | 5 |
| adaptive_batch | process_resident_memory_bytes | 11157504 | 10567680 … 11386880 | 5 |
| adaptive_batch | received | 6400 | 6400 … 6400 | 5 |
| adaptive_batch | rss_peak_bytes | 36519936 | 35897344 … 36700160 | 5 |
| adaptive_batch | seconds | 2 | 2 … 2 | 5 |
| adaptive_batch | target_rate | 100 | 100 … 100 | 5 |
| adaptive_batch | viewers | 32 | 32 … 32 | 5 |
| adaptive_batch | worst_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | achieved_rate | 99.94824554910163 | 99.93683367506524 … 99.94888449131454 | 5 |
| mixed_25pct_legacy | affected | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_p50_us | 384 | 342 … 1470 | 5 |
| mixed_25pct_legacy | cold_p99_us | 778 | 662 … 2373 | 5 |
| mixed_25pct_legacy | cold_rate | 100 | 100 … 100 | 5 |
| mixed_25pct_legacy | cold_received | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_samples | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_unique | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_count | 600 | 600 … 600 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_sum | 0.0025638669999999988 | 0.0007324499999999993 … 0.005190602999999998 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="single"} | 10400 | 10400 … 10400 | 5 |
| mixed_25pct_legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | expected | 6400 | 6400 … 6400 | 5 |
| mixed_25pct_legacy | frames | 6400 | 6400 … 6400 | 5 |
| mixed_25pct_legacy | go_gc_duration_seconds_sum | 0.000271792 | 0.000166458 … 0.000565251 | 5 |
| mixed_25pct_legacy | go_goroutines | 1 | 0 … 2 | 5 |
| mixed_25pct_legacy | go_memstats_alloc_bytes_total | 12631136 | 10080200 … 14005568 | 5 |
| mixed_25pct_legacy | goroutines_peak | 179 | 178 … 179 | 5 |
| mixed_25pct_legacy | hot_p50_us | 408 | 367 … 1490 | 5 |
| mixed_25pct_legacy | hot_p99_us | 982 | 718 … 2633 | 5 |
| mixed_25pct_legacy | hot_samples | 1600 | 1600 … 1600 | 5 |
| mixed_25pct_legacy | missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | payload_bytes | 2194912 | 2194272 … 2195232 | 5 |
| mixed_25pct_legacy | process_cpu_seconds_total | 0.14366700000000002 | 0.136862 … 0.390378 | 5 |
| mixed_25pct_legacy | process_resident_memory_bytes | 10764288 | 10403840 … 11665408 | 5 |
| mixed_25pct_legacy | received | 6400 | 6400 … 6400 | 5 |
| mixed_25pct_legacy | rss_peak_bytes | 36241408 | 35684352 … 36749312 | 5 |
| mixed_25pct_legacy | seconds | 2 | 2 … 2 | 5 |
| mixed_25pct_legacy | target_rate | 100 | 100 … 100 | 5 |
| mixed_25pct_legacy | viewers | 32 | 32 … 32 | 5 |
| mixed_25pct_legacy | worst_missing | 0 | 0 … 0 | 5 |

## fanout_sweep medium · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| legacy | achieved_rate | 399.95604643032146 | 399.9555217463376 … 399.96461133117174 | 5 |
| legacy | affected | 0 | 0 … 0 | 5 |
| legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| legacy | cold_gaps | 0 | 0 … 0 | 5 |
| legacy | cold_missing | 0 | 0 … 0 | 5 |
| legacy | cold_p50_us | 1165 | 364 … 1337 | 5 |
| legacy | cold_p99_us | 2735 | 1252 … 3097 | 5 |
| legacy | cold_rate | 100 | 100 … 100 | 5 |
| legacy | cold_received | 200 | 200 … 200 | 5 |
| legacy | cold_samples | 200 | 200 … 200 | 5 |
| legacy | cold_unique | 200 | 200 … 200 | 5 |
| legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_count | 1000 | 1000 … 1000 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_sum | 0.004155418999999998 | 0.0015287030000000021 … 0.013434416999999997 | 5 |
| legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="single"} | 80800 | 80800 … 80800 | 5 |
| legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| legacy | duplicates | 0 | 0 … 0 | 5 |
| legacy | expected | 76800 | 76800 … 76800 | 5 |
| legacy | frames | 76800 | 76800 … 76800 | 5 |
| legacy | go_gc_duration_seconds_sum | 0.000346128 | 0.00017804 … 0.00055779 | 5 |
| legacy | go_goroutines | 1 | -1 … 2 | 5 |
| legacy | go_memstats_alloc_bytes_total | 19549240 | 19364720 … 21652528 | 5 |
| legacy | goroutines_peak | 307 | 307 … 307 | 5 |
| legacy | hot_p50_us | 1085 | 395 … 1230 | 5 |
| legacy | hot_p99_us | 2889 | 1527 … 3583 | 5 |
| legacy | hot_samples | 6400 | 6400 … 6400 | 5 |
| legacy | missing | 0 | 0 … 0 | 5 |
| legacy | payload_bytes | 26378592 | 26373504 … 26389152 | 5 |
| legacy | process_cpu_seconds_total | 1.273327 | 0.621454 … 1.310922 | 5 |
| legacy | process_resident_memory_bytes | 14925824 | 14434304 … 16384000 | 5 |
| legacy | received | 76800 | 76800 … 76800 | 5 |
| legacy | rss_peak_bytes | 40206336 | 39452672 … 41746432 | 5 |
| legacy | seconds | 2 | 2 … 2 | 5 |
| legacy | target_rate | 400 | 400 … 400 | 5 |
| legacy | viewers | 96 | 96 … 96 | 5 |
| legacy | worst_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | achieved_rate | 399.9581959693991 | 399.9467404923822 … 399.9700938361138 | 5 |
| adaptive_batch | affected | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_p50_us | 1190 | 1098 … 1309 | 5 |
| adaptive_batch | cold_p99_us | 2536 | 2199 … 9385 | 5 |
| adaptive_batch | cold_rate | 100 | 100 … 100 | 5 |
| adaptive_batch | cold_received | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_samples | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_unique | 200 | 200 … 200 | 5 |
| adaptive_batch | cursor_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_count | 1000 | 1000 … 1000 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_sum | 0.003382732 | 0.003064955000000003 … 0.0051353679999999995 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="single"} | 80800 | 80800 … 80800 | 5 |
| adaptive_batch | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| adaptive_batch | duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | expected | 76800 | 76800 … 76800 | 5 |
| adaptive_batch | frames | 76800 | 76800 … 76800 | 5 |
| adaptive_batch | go_gc_duration_seconds_sum | 0.000429917 | 0.00039495900000000007 … 0.0006246649999999999 | 5 |
| adaptive_batch | go_goroutines | 0 | 0 … 1 | 5 |
| adaptive_batch | go_memstats_alloc_bytes_total | 20167656 | 19443488 … 20526848 | 5 |
| adaptive_batch | goroutines_peak | 307 | 306 … 307 | 5 |
| adaptive_batch | hot_p50_us | 1146 | 1087 … 1238 | 5 |
| adaptive_batch | hot_p99_us | 2847 | 2624 … 6757 | 5 |
| adaptive_batch | hot_samples | 6400 | 6400 … 6400 | 5 |
| adaptive_batch | missing | 0 | 0 … 0 | 5 |
| adaptive_batch | payload_bytes | 26375232 | 26371968 … 26380032 | 5 |
| adaptive_batch | process_cpu_seconds_total | 1.251758 | 1.217806 … 1.308173 | 5 |
| adaptive_batch | process_resident_memory_bytes | 15007744 | 14827520 … 15745024 | 5 |
| adaptive_batch | received | 76800 | 76800 … 76800 | 5 |
| adaptive_batch | rss_peak_bytes | 40288256 | 40091648 … 41074688 | 5 |
| adaptive_batch | seconds | 2 | 2 … 2 | 5 |
| adaptive_batch | target_rate | 400 | 400 … 400 | 5 |
| adaptive_batch | viewers | 96 | 96 … 96 | 5 |
| adaptive_batch | worst_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | achieved_rate | 399.95308050424336 | 399.9264967093168 … 399.9703106038142 | 5 |
| mixed_25pct_legacy | affected | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_p50_us | 1252 | 679 … 1322 | 5 |
| mixed_25pct_legacy | cold_p99_us | 2947 | 2517 … 7224 | 5 |
| mixed_25pct_legacy | cold_rate | 100 | 100 … 100 | 5 |
| mixed_25pct_legacy | cold_received | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_samples | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_unique | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_count | 1800 | 1800 … 1800 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_sum | 0.025053036999999972 | 0.014577024999999992 … 0.027335909999999998 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="single"} | 80800 | 80800 … 80800 | 5 |
| mixed_25pct_legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | expected | 76800 | 76800 … 76800 | 5 |
| mixed_25pct_legacy | frames | 76800 | 76800 … 76800 | 5 |
| mixed_25pct_legacy | go_gc_duration_seconds_sum | 0.000408375 | 0.000259584 … 0.000735459 | 5 |
| mixed_25pct_legacy | go_goroutines | 1 | 0 … 2 | 5 |
| mixed_25pct_legacy | go_memstats_alloc_bytes_total | 19673656 | 18412152 … 22394472 | 5 |
| mixed_25pct_legacy | goroutines_peak | 307 | 306 … 307 | 5 |
| mixed_25pct_legacy | hot_p50_us | 1236 | 709 … 1265 | 5 |
| mixed_25pct_legacy | hot_p99_us | 2609 | 2478 … 7144 | 5 |
| mixed_25pct_legacy | hot_samples | 6400 | 6400 … 6400 | 5 |
| mixed_25pct_legacy | missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | payload_bytes | 26376768 | 26370816 … 26384832 | 5 |
| mixed_25pct_legacy | process_cpu_seconds_total | 1.3127449999999998 | 1.0188979999999999 … 1.339192 | 5 |
| mixed_25pct_legacy | process_resident_memory_bytes | 15417344 | 14336000 … 16384000 | 5 |
| mixed_25pct_legacy | received | 76800 | 76800 … 76800 | 5 |
| mixed_25pct_legacy | rss_peak_bytes | 40697856 | 39485440 … 41631744 | 5 |
| mixed_25pct_legacy | seconds | 2 | 2 … 2 | 5 |
| mixed_25pct_legacy | target_rate | 400 | 400 … 400 | 5 |
| mixed_25pct_legacy | viewers | 96 | 96 … 96 | 5 |
| mixed_25pct_legacy | worst_missing | 0 | 0 … 0 | 5 |

## fanout_sweep high · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| legacy | achieved_rate | 699.96039819056 | 699.9531509356882 … 699.968709848739 | 5 |
| legacy | affected | 0 | 0 … 0 | 5 |
| legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| legacy | cold_gaps | 0 | 0 … 0 | 5 |
| legacy | cold_missing | 0 | 0 … 0 | 5 |
| legacy | cold_p50_us | 472 | 398 … 557 | 5 |
| legacy | cold_p99_us | 3275 | 1784 … 8467 | 5 |
| legacy | cold_rate | 100 | 100 … 100 | 5 |
| legacy | cold_received | 200 | 200 … 200 | 5 |
| legacy | cold_samples | 200 | 200 … 200 | 5 |
| legacy | cold_unique | 200 | 200 … 200 | 5 |
| legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_count | 1600 | 1600 … 1600 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_sum | 0.003983426999999946 | 0.0011515740000000044 … 0.017154662000000247 | 5 |
| legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="single"} | 317600 | 317600 … 317600 | 5 |
| legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| legacy | duplicates | 0 | 0 … 0 | 5 |
| legacy | expected | 313600 | 313600 … 313600 | 5 |
| legacy | frames | 313600 | 313600 … 313600 | 5 |
| legacy | go_gc_duration_seconds_sum | 0.000255584 | 0.00022479300000000004 … 0.001321583 | 5 |
| legacy | go_goroutines | 1 | 1 … 1 | 5 |
| legacy | go_memstats_alloc_bytes_total | 27882520 | 23688488 … 28986368 | 5 |
| legacy | goroutines_peak | 563 | 562 … 563 | 5 |
| legacy | hot_p50_us | 610 | 576 … 681 | 5 |
| legacy | hot_p99_us | 5238 | 3501 … 10120 | 5 |
| legacy | hot_samples | 11200 | 11200 … 11200 | 5 |
| legacy | missing | 0 | 0 … 0 | 5 |
| legacy | payload_bytes | 107821728 | 107809184 … 107829344 | 5 |
| legacy | process_cpu_seconds_total | 2.5383 | 2.459957 … 2.610716 | 5 |
| legacy | process_resident_memory_bytes | 22380544 | 21479424 … 23969792 | 5 |
| legacy | received | 313600 | 313600 … 313600 | 5 |
| legacy | rss_peak_bytes | 47857664 | 46923776 … 49332224 | 5 |
| legacy | seconds | 2 | 2 … 2 | 5 |
| legacy | target_rate | 700 | 700 … 700 | 5 |
| legacy | viewers | 224 | 224 … 224 | 5 |
| legacy | worst_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | achieved_rate | 699.9355077923258 | 699.9289028218889 … 699.989096119852 | 5 |
| adaptive_batch | affected | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_p50_us | 479 | 460 … 483 | 5 |
| adaptive_batch | cold_p99_us | 3298 | 3002 … 4792 | 5 |
| adaptive_batch | cold_rate | 100 | 100 … 100 | 5 |
| adaptive_batch | cold_received | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_samples | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_unique | 200 | 200 … 200 | 5 |
| adaptive_batch | cursor_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_count | 642 | 637 … 652 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_sum | 0.0011622109999999976 | 0.0009340209999999987 … 0.001684117 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="batch"} | 68992 | 67200 … 69440 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="single"} | 34016 | 32448 … 38048 | 5 |
| adaptive_batch | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| adaptive_batch | duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | expected | 313600 | 313600 … 313600 | 5 |
| adaptive_batch | frames | 99008 | 97888 … 101248 | 5 |
| adaptive_batch | go_gc_duration_seconds_sum | 0.00045521 | 0.00032233299999999995 … 0.0009662099999999999 | 5 |
| adaptive_batch | go_goroutines | 1 | 1 … 2 | 5 |
| adaptive_batch | go_memstats_alloc_bytes_total | 28853808 | 27952488 … 29486080 | 5 |
| adaptive_batch | goroutines_peak | 563 | 563 … 563 | 5 |
| adaptive_batch | hot_p50_us | 4010 | 3978 … 4192 | 5 |
| adaptive_batch | hot_p99_us | 8518 | 8226 … 8926 | 5 |
| adaptive_batch | hot_samples | 11200 | 11200 … 11200 | 5 |
| adaptive_batch | missing | 0 | 0 … 0 | 5 |
| adaptive_batch | payload_bytes | 108179680 | 108167136 … 108184608 | 5 |
| adaptive_batch | process_cpu_seconds_total | 1.331456 | 1.227908 … 1.394703 | 5 |
| adaptive_batch | process_resident_memory_bytes | 22642688 | 21757952 … 23822336 | 5 |
| adaptive_batch | received | 313600 | 313600 … 313600 | 5 |
| adaptive_batch | rss_peak_bytes | 47955968 | 47349760 … 48922624 | 5 |
| adaptive_batch | seconds | 2 | 2 … 2 | 5 |
| adaptive_batch | target_rate | 700 | 700 … 700 | 5 |
| adaptive_batch | viewers | 224 | 224 … 224 | 5 |
| adaptive_batch | worst_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | achieved_rate | 699.9546676359375 | 699.9354934449702 … 699.9710137003434 | 5 |
| mixed_25pct_legacy | affected | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_p50_us | 549 | 483 … 598 | 5 |
| mixed_25pct_legacy | cold_p99_us | 2565 | 2133 … 16012 | 5 |
| mixed_25pct_legacy | cold_rate | 100 | 100 … 100 | 5 |
| mixed_25pct_legacy | cold_received | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_samples | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_unique | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_count | 1996 | 1990 … 2001 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_sum | 0.004246842000000021 | 0.004028952000000013 … 0.012134173999999968 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="batch"} | 55272 | 54432 … 55944 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="single"} | 94496 | 91976 … 94664 | 5 |
| mixed_25pct_legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | expected | 313600 | 313600 … 313600 | 5 |
| mixed_25pct_legacy | frames | 144928 | 143920 … 145768 | 5 |
| mixed_25pct_legacy | go_gc_duration_seconds_sum | 0.000412833 | 0.00027870799999999996 … 0.0007394589999999999 | 5 |
| mixed_25pct_legacy | go_goroutines | 1 | 1 … 1 | 5 |
| mixed_25pct_legacy | go_memstats_alloc_bytes_total | 28840680 | 25698360 … 30056240 | 5 |
| mixed_25pct_legacy | goroutines_peak | 563 | 562 … 569 | 5 |
| mixed_25pct_legacy | hot_p50_us | 3188 | 3158 … 3226 | 5 |
| mixed_25pct_legacy | hot_p99_us | 7526 | 7332 … 16795 | 5 |
| mixed_25pct_legacy | hot_samples | 11200 | 11200 … 11200 | 5 |
| mixed_25pct_legacy | missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | payload_bytes | 108102064 | 108085600 … 108108000 | 5 |
| mixed_25pct_legacy | process_cpu_seconds_total | 1.657621 | 1.442734 … 1.687274 | 5 |
| mixed_25pct_legacy | process_resident_memory_bytes | 23838720 | 22675456 … 24854528 | 5 |
| mixed_25pct_legacy | received | 313600 | 313600 … 313600 | 5 |
| mixed_25pct_legacy | rss_peak_bytes | 49348608 | 47906816 … 50151424 | 5 |
| mixed_25pct_legacy | seconds | 2 | 2 … 2 | 5 |
| mixed_25pct_legacy | target_rate | 700 | 700 … 700 | 5 |
| mixed_25pct_legacy | viewers | 224 | 224 … 224 | 5 |
| mixed_25pct_legacy | worst_missing | 0 | 0 … 0 | 5 |

## isolation_stalled room_shards · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| single_shard | achieved_rate | 599.9398810244224 | 599.9377565577469 … 599.9604276101289 | 5 |
| single_shard | affected | 0 | 0 … 0 | 5 |
| single_shard | cold_duplicates | 0 | 0 … 0 | 5 |
| single_shard | cold_gaps | 0 | 0 … 0 | 5 |
| single_shard | cold_missing | 0 | 0 … 0 | 5 |
| single_shard | cold_p50_us | 583 | 560 … 620 | 5 |
| single_shard | cold_p99_us | 2760 | 2220 … 4086 | 5 |
| single_shard | cold_rate | 100 | 100 … 100 | 5 |
| single_shard | cold_received | 200 | 200 … 200 | 5 |
| single_shard | cold_samples | 200 | 200 … 200 | 5 |
| single_shard | cold_unique | 200 | 200 … 200 | 5 |
| single_shard | cursor_gaps | 0 | 0 … 0 | 5 |
| single_shard | danmaku_delivery_queue_wait_seconds_count | 605 | 597 … 609 | 5 |
| single_shard | danmaku_delivery_queue_wait_seconds_sum | 0.001831135000000001 | 0.0016505889999999968 … 0.002361323999999998 | 5 |
| single_shard | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| single_shard | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| single_shard | danmaku_fanout_frames_total{mode="batch"} | 52960 | 52160 … 53760 | 5 |
| single_shard | danmaku_fanout_frames_total{mode="single"} | 15844 | 13760 … 16960 | 5 |
| single_shard | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| single_shard | duplicates | 0 | 0 … 0 | 5 |
| single_shard | expected | 192000 | 192000 … 192000 | 5 |
| single_shard | frames | 64000 | 63520 … 65440 | 5 |
| single_shard | go_gc_duration_seconds_sum | 0.000429586 | 0.00036196 … 0.000695085 | 5 |
| single_shard | go_goroutines | 18 | 11 … 20 | 5 |
| single_shard | go_memstats_alloc_bytes_total | 27825080 | 27144656 … 28701256 | 5 |
| single_shard | goroutines_peak | 468 | 467 … 468 | 5 |
| single_shard | hot_p50_us | 4086 | 4044 … 4165 | 5 |
| single_shard | hot_p99_us | 7650 | 7277 … 9550 | 5 |
| single_shard | hot_samples | 9600 | 9600 … 9600 | 5 |
| single_shard | missing | 0 | 0 … 0 | 5 |
| single_shard | payload_bytes | 66220160 | 66210720 … 66224320 | 5 |
| single_shard | process_cpu_seconds_total | 1.235059 | 1.2042380000000001 … 1.3179210000000001 | 5 |
| single_shard | process_resident_memory_bytes | 21233664 | 20480000 … 23101440 | 5 |
| single_shard | received | 192000 | 192000 … 192000 | 5 |
| single_shard | rss_peak_bytes | 46350336 | 45809664 … 48250880 | 5 |
| single_shard | seconds | 2 | 2 … 2 | 5 |
| single_shard | target_rate | 600 | 600 … 600 | 5 |
| single_shard | viewers | 160 | 160 … 160 | 5 |
| single_shard | worst_missing | 0 | 0 … 0 | 5 |
| default_shards | achieved_rate | 599.9482170695056 | 599.9343447851317 … 599.9875626578179 | 5 |
| default_shards | affected | 0 | 0 … 0 | 5 |
| default_shards | cold_duplicates | 0 | 0 … 0 | 5 |
| default_shards | cold_gaps | 0 | 0 … 0 | 5 |
| default_shards | cold_missing | 0 | 0 … 0 | 5 |
| default_shards | cold_p50_us | 587 | 562 … 654 | 5 |
| default_shards | cold_p99_us | 2151 | 1720 … 2565 | 5 |
| default_shards | cold_rate | 100 | 100 … 100 | 5 |
| default_shards | cold_received | 200 | 200 … 200 | 5 |
| default_shards | cold_samples | 200 | 200 … 200 | 5 |
| default_shards | cold_unique | 200 | 200 … 200 | 5 |
| default_shards | cursor_gaps | 0 | 0 … 0 | 5 |
| default_shards | danmaku_delivery_queue_wait_seconds_count | 600 | 583 … 611 | 5 |
| default_shards | danmaku_delivery_queue_wait_seconds_sum | 0.0015094620000000013 | 0.0009448709999999998 … 0.0022970300000000007 | 5 |
| default_shards | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| default_shards | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| default_shards | danmaku_fanout_frames_total{mode="batch"} | 53120 | 51840 … 54400 | 5 |
| default_shards | danmaku_fanout_frames_total{mode="single"} | 15040 | 10880 … 17761 | 5 |
| default_shards | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| default_shards | duplicates | 0 | 0 … 0 | 5 |
| default_shards | expected | 192000 | 192000 … 192000 | 5 |
| default_shards | frames | 64000 | 61280 … 65600 | 5 |
| default_shards | go_gc_duration_seconds_sum | 0.000401959 | 0.000307293 … 0.000516918 | 5 |
| default_shards | go_goroutines | 13 | 2 … 23 | 5 |
| default_shards | go_memstats_alloc_bytes_total | 29110184 | 27120944 … 29646104 | 5 |
| default_shards | goroutines_peak | 499 | 498 … 499 | 5 |
| default_shards | hot_p50_us | 4099 | 4025 … 4166 | 5 |
| default_shards | hot_p99_us | 7533 | 7324 … 7739 | 5 |
| default_shards | hot_samples | 9600 | 9600 … 9600 | 5 |
| default_shards | missing | 0 | 0 … 0 | 5 |
| default_shards | payload_bytes | 66214720 | 66201280 … 66225440 | 5 |
| default_shards | process_cpu_seconds_total | 1.190846 | 1.145945 … 1.316514 | 5 |
| default_shards | process_resident_memory_bytes | 21757952 | 21430272 … 22282240 | 5 |
| default_shards | received | 192000 | 192000 … 192000 | 5 |
| default_shards | rss_peak_bytes | 47087616 | 46841856 … 47677440 | 5 |
| default_shards | seconds | 2 | 2 … 2 | 5 |
| default_shards | target_rate | 600 | 600 … 600 | 5 |
| default_shards | viewers | 160 | 160 … 160 | 5 |
| default_shards | worst_missing | 0 | 0 … 0 | 5 |

## isolation_stalled slow_kick · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| kick_disabled | achieved_rate | 599.9724804955964 | 599.8008577513368 … 599.9808089471804 | 5 |
| kick_disabled | affected | 0 | 0 … 0 | 5 |
| kick_disabled | cold_duplicates | 0 | 0 … 0 | 5 |
| kick_disabled | cold_gaps | 0 | 0 … 0 | 5 |
| kick_disabled | cold_missing | 0 | 0 … 0 | 5 |
| kick_disabled | cold_p50_us | 379 | 337 … 389 | 5 |
| kick_disabled | cold_p99_us | 2593 | 2346 … 5707 | 5 |
| kick_disabled | cold_rate | 100 | 100 … 100 | 5 |
| kick_disabled | cold_received | 600 | 600 … 600 | 5 |
| kick_disabled | cold_samples | 600 | 600 … 600 | 5 |
| kick_disabled | cold_unique | 600 | 600 … 600 | 5 |
| kick_disabled | cursor_gaps | 0 | 0 … 0 | 5 |
| kick_disabled | danmaku_delivery_queue_wait_seconds_count | 5418 | 5247 … 5426 | 5 |
| kick_disabled | danmaku_delivery_queue_wait_seconds_sum | 0.00964479900000003 | 0.008972019000000006 … 0.01286401200000003 | 5 |
| kick_disabled | danmaku_dropped_messages_total{reason="client_full"} | 11992 | 11328 … 11992 | 5 |
| kick_disabled | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| kick_disabled | danmaku_fanout_frames_total{mode="batch"} | 150400 | 148800 … 151520 | 5 |
| kick_disabled | danmaku_fanout_frames_total{mode="single"} | 58057 | 56449 … 60774 | 5 |
| kick_disabled | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| kick_disabled | duplicates | 0 | 0 … 0 | 5 |
| kick_disabled | expected | 576000 | 576000 … 576000 | 5 |
| kick_disabled | frames | 194880 | 194240 … 196160 | 5 |
| kick_disabled | go_gc_duration_seconds_sum | 0.001025579 | 0.000715702 … 0.002009664 | 5 |
| kick_disabled | go_goroutines | 1 | 0 … 3 | 5 |
| kick_disabled | go_memstats_alloc_bytes_total | 134500288 | 131332400 … 135946216 | 5 |
| kick_disabled | goroutines_peak | 507 | 507 … 507 | 5 |
| kick_disabled | hot_p50_us | 4083 | 4078 … 4153 | 5 |
| kick_disabled | hot_p99_us | 8096 | 7970 … 12537 | 5 |
| kick_disabled | hot_samples | 28800 | 28800 … 28800 | 5 |
| kick_disabled | missing | 0 | 0 … 0 | 5 |
| kick_disabled | payload_bytes | 890799200 | 890794720 … 890825280 | 5 |
| kick_disabled | process_cpu_seconds_total | 2.5471890000000004 | 2.484941 … 2.55754 | 5 |
| kick_disabled | process_resident_memory_bytes | 35307520 | 34652160 … 36061184 | 5 |
| kick_disabled | received | 576000 | 576000 … 576000 | 5 |
| kick_disabled | rss_peak_bytes | 60850176 | 59867136 … 61521920 | 5 |
| kick_disabled | seconds | 6 | 6 … 6 | 5 |
| kick_disabled | target_rate | 600 | 600 … 600 | 5 |
| kick_disabled | viewers | 160 | 160 … 160 | 5 |
| kick_disabled | worst_missing | 0 | 0 … 0 | 5 |
| kick_enabled | achieved_rate | 599.978354914211 | 599.9752968504512 … 599.9932376095508 | 5 |
| kick_enabled | affected | 0 | 0 … 0 | 5 |
| kick_enabled | cold_duplicates | 0 | 0 … 0 | 5 |
| kick_enabled | cold_gaps | 0 | 0 … 0 | 5 |
| kick_enabled | cold_missing | 0 | 0 … 0 | 5 |
| kick_enabled | cold_p50_us | 372 | 353 … 378 | 5 |
| kick_enabled | cold_p99_us | 2385 | 2148 … 8084 | 5 |
| kick_enabled | cold_rate | 100 | 100 … 100 | 5 |
| kick_enabled | cold_received | 600 | 600 … 600 | 5 |
| kick_enabled | cold_samples | 600 | 600 … 600 | 5 |
| kick_enabled | cold_unique | 600 | 600 … 600 | 5 |
| kick_enabled | cursor_gaps | 0 | 0 … 0 | 5 |
| kick_enabled | danmaku_delivery_queue_wait_seconds_count | 2420 | 2409 … 2433 | 5 |
| kick_enabled | danmaku_delivery_queue_wait_seconds_sum | 0.0035909170000000593 | 0.0029470050000000325 … 0.005493914000000036 | 5 |
| kick_enabled | danmaku_dropped_messages_total{reason="client_full"} | 32 | 32 … 32 | 5 |
| kick_enabled | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| kick_enabled | danmaku_fanout_frames_total{mode="batch"} | 151520 | 147840 … 153600 | 5 |
| kick_enabled | danmaku_fanout_frames_total{mode="single"} | 55331 | 52452 … 60932 | 5 |
| kick_enabled | danmaku_kicked_slow_clients_total | 4 | 4 … 4 | 5 |
| kick_enabled | duplicates | 0 | 0 … 0 | 5 |
| kick_enabled | expected | 576000 | 576000 … 576000 | 5 |
| kick_enabled | frames | 193120 | 191360 … 195040 | 5 |
| kick_enabled | go_gc_duration_seconds_sum | 0.001194 | 0.000803998 … 0.002972794 | 5 |
| kick_enabled | go_goroutines | 1 | 1 … 2 | 5 |
| kick_enabled | go_memstats_alloc_bytes_total | 133881816 | 132158736 … 134845056 | 5 |
| kick_enabled | goroutines_peak | 506 | 506 … 507 | 5 |
| kick_enabled | hot_p50_us | 4125 | 4075 … 4195 | 5 |
| kick_enabled | hot_p99_us | 8053 | 7869 … 26096 | 5 |
| kick_enabled | hot_samples | 28800 | 28800 … 28800 | 5 |
| kick_enabled | missing | 0 | 0 … 0 | 5 |
| kick_enabled | payload_bytes | 890789120 | 890783040 … 890832320 | 5 |
| kick_enabled | process_cpu_seconds_total | 2.502488 | 2.424134 … 2.611246 | 5 |
| kick_enabled | process_resident_memory_bytes | 34799616 | 33030144 … 35471360 | 5 |
| kick_enabled | received | 576000 | 576000 … 576000 | 5 |
| kick_enabled | rss_peak_bytes | 59899904 | 58540032 … 60784640 | 5 |
| kick_enabled | seconds | 6 | 6 … 6 | 5 |
| kick_enabled | target_rate | 600 | 600 … 600 | 5 |
| kick_enabled | viewers | 160 | 160 … 160 | 5 |
| kick_enabled | worst_missing | 0 | 0 … 0 | 5 |

## migration_strategies · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| crash | complete_viewers | 144 | 144 … 144 | 5 |
| crash | connect_peak_qps | 330 | 320 … 370 | 5 |
| crash | danmaku_delivery_queue_wait_seconds_count | 24 | 24 … 24 | 5 |
| crash | danmaku_delivery_queue_wait_seconds_sum | 0.000143667 | 0.00012608699999999996 … 0.00017191799999999996 | 5 |
| crash | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| crash | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| crash | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| crash | danmaku_fanout_frames_total{mode="single"} | 3211 | 3207 … 3217 | 5 |
| crash | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| crash | drain_ms | 452 | 448 … 456 | 5 |
| crash | events | 24 | 24 … 24 | 5 |
| crash | go_gc_duration_seconds_sum | 0.000617584 | 0.00030379100000000003 … 0.000708125 | 5 |
| crash | go_goroutines | 5 | 4 … 6 | 5 |
| crash | go_memstats_alloc_bytes_total | 7544624 | 7389384 … 8174352 | 5 |
| crash | invited | 0 | 0 … 0 | 5 |
| crash | last_handoff_ms | 452 | 448 … 456 | 5 |
| crash | migrating_delivery_p50_us | 2332 | 2164 … 2395 | 5 |
| crash | migrating_delivery_p95_us | 171150 | 154792 … 174230 | 5 |
| crash | migrating_delivery_p99_us | 391619 | 377697 … 405617 | 5 |
| crash | migrating_delivery_samples | 1584 | 1584 … 1584 | 5 |
| crash | moved | 72 | 72 … 72 | 5 |
| crash | peak_attempts_per_sec | 72 | 72 … 72 | 5 |
| crash | process_cpu_seconds_total | 0.17850100000000002 | 0.172022 … 0.185251 | 5 |
| crash | process_resident_memory_bytes | 10387456 | 9912320 … 11091968 | 5 |
| crash | recent_peak_qps | 330 | 320 … 370 | 5 |
| crash | recovery_p50_ms | 320 | 285 … 329 | 5 |
| crash | recovery_p95_ms | 438 | 428 … 444 | 5 |
| crash | recovery_p99_ms | 449 | 443 … 452 | 5 |
| crash | recovery_samples | 72 | 72 … 72 | 5 |
| crash | resident_delivery_p50_us | 2298 | 2109 … 2384 | 5 |
| crash | resident_delivery_p95_us | 3039 | 2812 … 3454 | 5 |
| crash | resident_delivery_p99_ms | 3 | 2 … 4 | 5 |
| crash | resident_delivery_p99_us | 3684 | 2981 … 4225 | 5 |
| crash | resident_delivery_samples | 1584 | 1584 … 1584 | 5 |
| crash | resident_during_handoff_p50_us | 2110 | 1400 … 2532 | 5 |
| crash | resident_during_handoff_p95_us | 2805 | 2218 … 3100 | 5 |
| crash | resident_during_handoff_p99_us | 2822 | 2247 … 3114 | 5 |
| crash | resident_during_handoff_samples | 144 | 144 … 144 | 5 |
| crash | source_exit_ms | 3.976 | 3.247 … 5.368 | 5 |
| crash | viewers | 144 | 144 … 144 | 5 |
| fixed | complete_viewers | 144 | 144 … 144 | 5 |
| fixed | connect_peak_qps | 390 | 350 … 480 | 5 |
| fixed | danmaku_delivery_queue_wait_seconds_count | 24 | 24 … 24 | 5 |
| fixed | danmaku_delivery_queue_wait_seconds_sum | 0.000144252 | 0.00012583500000000003 … 0.000177874 | 5 |
| fixed | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| fixed | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| fixed | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| fixed | danmaku_fanout_frames_total{mode="single"} | 3240 | 3240 … 3240 | 5 |
| fixed | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| fixed | drain_ms | 404.901 | 402.078 … 405.208 | 5 |
| fixed | events | 24 | 24 … 24 | 5 |
| fixed | go_gc_duration_seconds_sum | 0.000529042 | 0.00042595799999999994 … 0.0011955829999999998 | 5 |
| fixed | go_goroutines | 26 | 26 … 26 | 5 |
| fixed | go_memstats_alloc_bytes_total | 7500768 | 6999272 … 7581944 | 5 |
| fixed | invited | 72 | 72 … 72 | 5 |
| fixed | last_handoff_ms | 307 | 306 … 309 | 5 |
| fixed | migrating_delivery_p50_us | 2227 | 2101 … 2441 | 5 |
| fixed | migrating_delivery_p95_us | 3314 | 2997 … 3725 | 5 |
| fixed | migrating_delivery_p99_us | 3748 | 3501 … 4321 | 5 |
| fixed | migrating_delivery_samples | 1584 | 1584 … 1584 | 5 |
| fixed | moved | 72 | 72 … 72 | 5 |
| fixed | peak_attempts_per_sec | 72 | 72 … 72 | 5 |
| fixed | process_cpu_seconds_total | 0.135795 | 0.12884 … 0.140583 | 5 |
| fixed | process_resident_memory_bytes | 11436032 | 10715136 … 12042240 | 5 |
| fixed | recent_peak_qps | 480 | 440 … 480 | 5 |
| fixed | recovery_p50_ms | 5 | 4 … 6 | 5 |
| fixed | recovery_p95_ms | 7 | 6 … 7 | 5 |
| fixed | recovery_p99_ms | 7 | 6 … 7 | 5 |
| fixed | recovery_samples | 72 | 72 … 72 | 5 |
| fixed | resident_delivery_p50_us | 2252 | 2106 … 2454 | 5 |
| fixed | resident_delivery_p95_us | 3346 | 3145 … 3677 | 5 |
| fixed | resident_delivery_p99_ms | 3 | 3 … 4 | 5 |
| fixed | resident_delivery_p99_us | 3878 | 3491 … 4302 | 5 |
| fixed | resident_delivery_samples | 1584 | 1584 … 1584 | 5 |
| fixed | resident_during_handoff_p50_us | 2517 | 1872 … 2796 | 5 |
| fixed | resident_during_handoff_p95_us | 2952 | 2526 … 4361 | 5 |
| fixed | resident_during_handoff_p99_us | 3034 | 2603 … 4786 | 5 |
| fixed | resident_during_handoff_samples | 144 | 144 … 144 | 5 |
| fixed | source_exit_ms | 404.901 | 402.078 … 405.208 | 5 |
| fixed | viewers | 144 | 144 … 144 | 5 |
| bounded | complete_viewers | 144 | 144 … 144 | 5 |
| bounded | connect_peak_qps | 80 | 80 … 80 | 5 |
| bounded | danmaku_delivery_queue_wait_seconds_count | 24 | 24 … 24 | 5 |
| bounded | danmaku_delivery_queue_wait_seconds_sum | 0.00013320999999999998 | 0.00012058299999999998 … 0.00014583 | 5 |
| bounded | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| bounded | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| bounded | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| bounded | danmaku_fanout_frames_total{mode="single"} | 3024 | 3024 … 3028 | 5 |
| bounded | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| bounded | drain_ms | 1905.346 | 1905.042 … 1905.689 | 5 |
| bounded | events | 24 | 24 … 24 | 5 |
| bounded | go_gc_duration_seconds_sum | 0.000539999 | 0.000477292 … 0.000683499 | 5 |
| bounded | go_goroutines | 6 | 5 … 6 | 5 |
| bounded | go_memstats_alloc_bytes_total | 7837648 | 7510920 … 8490288 | 5 |
| bounded | invited | 72 | 72 … 72 | 5 |
| bounded | last_handoff_ms | 1804 | 1803 … 1805 | 5 |
| bounded | migrating_delivery_p50_us | 2128 | 2046 … 2378 | 5 |
| bounded | migrating_delivery_p95_us | 2834 | 2771 … 3605 | 5 |
| bounded | migrating_delivery_p99_us | 3715 | 3068 … 5404 | 5 |
| bounded | migrating_delivery_samples | 1584 | 1584 … 1584 | 5 |
| bounded | moved | 72 | 72 … 72 | 5 |
| bounded | peak_attempts_per_sec | 36 | 36 … 36 | 5 |
| bounded | process_cpu_seconds_total | 0.17431799999999997 | 0.14241399999999999 … 0.17649199999999998 | 5 |
| bounded | process_resident_memory_bytes | 10633216 | 9895936 … 10682368 | 5 |
| bounded | recent_peak_qps | 80 | 80 … 80 | 5 |
| bounded | recovery_p50_ms | 3 | 2 … 3 | 5 |
| bounded | recovery_p95_ms | 5 | 4 … 5 | 5 |
| bounded | recovery_p99_ms | 5 | 4 … 6 | 5 |
| bounded | recovery_samples | 72 | 72 … 72 | 5 |
| bounded | resident_delivery_p50_us | 2139 | 2045 … 2350 | 5 |
| bounded | resident_delivery_p95_us | 2962 | 2661 … 3580 | 5 |
| bounded | resident_delivery_p99_ms | 3 | 3 … 5 | 5 |
| bounded | resident_delivery_p99_us | 3702 | 3259 … 5142 | 5 |
| bounded | resident_delivery_samples | 1584 | 1584 … 1584 | 5 |
| bounded | resident_during_handoff_p50_us | 2127 | 1820 … 2434 | 5 |
| bounded | resident_during_handoff_p95_us | 3162 | 2251 … 4765 | 5 |
| bounded | resident_during_handoff_p99_us | 3243 | 2330 … 5329 | 5 |
| bounded | resident_during_handoff_samples | 576 | 576 … 576 | 5 |
| bounded | source_exit_ms | 1905.346 | 1905.042 … 1905.689 | 5 |
| bounded | viewers | 144 | 144 … 144 | 5 |

## history_pagination page_1 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| offset_baseline | p50_us | 383.042 | 366.583 … 385.792 | 5 |
| offset_baseline | p95_us | 402.834 | 390.041 … 411.292 | 5 |
| offset_baseline | p999_us | 402.834 | 390.041 … 411.292 | 5 |
| offset_baseline | p99_us | 402.834 | 390.041 … 411.292 | 5 |
| offset_baseline | samples | 6 | 6 … 6 | 5 |
| cursor_before_id_prod | p50_us | 387.416 | 367.667 … 396.916 | 5 |
| cursor_before_id_prod | p95_us | 410.375 | 377.666 … 451.666 | 5 |
| cursor_before_id_prod | p999_us | 410.375 | 377.666 … 451.666 | 5 |
| cursor_before_id_prod | p99_us | 410.375 | 377.666 … 451.666 | 5 |
| cursor_before_id_prod | samples | 6 | 6 … 6 | 5 |

## history_pagination page_200 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| offset_baseline | p50_us | 9304.458 | 9054.292 … 9366.416 | 5 |
| offset_baseline | p95_us | 9647.917 | 9383.459 … 20573.833 | 5 |
| offset_baseline | p999_us | 9647.917 | 9383.459 … 20573.833 | 5 |
| offset_baseline | p99_us | 9647.917 | 9383.459 … 20573.833 | 5 |
| offset_baseline | samples | 6 | 6 … 6 | 5 |
| cursor_before_id_prod | p50_us | 416.583 | 380.5 … 425.166 | 5 |
| cursor_before_id_prod | p95_us | 462.833 | 406.208 … 494.666 | 5 |
| cursor_before_id_prod | p999_us | 462.833 | 406.208 … 494.666 | 5 |
| cursor_before_id_prod | p99_us | 462.833 | 406.208 … 494.666 | 5 |
| cursor_before_id_prod | samples | 6 | 6 … 6 | 5 |

## history_pagination page_1000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| offset_baseline | p50_us | 37296.708 | 36944.417 … 38090.209 | 5 |
| offset_baseline | p95_us | 38442.709 | 37322.25 … 45558.208 | 5 |
| offset_baseline | p999_us | 38442.709 | 37322.25 … 45558.208 | 5 |
| offset_baseline | p99_us | 38442.709 | 37322.25 … 45558.208 | 5 |
| offset_baseline | samples | 6 | 6 … 6 | 5 |
| cursor_before_id_prod | p50_us | 408.458 | 400.042 … 422.583 | 5 |
| cursor_before_id_prod | p95_us | 470.833 | 443 … 524.833 | 5 |
| cursor_before_id_prod | p999_us | 470.833 | 443 … 524.833 | 5 |
| cursor_before_id_prod | p99_us | 470.833 | 443 … 524.833 | 5 |
| cursor_before_id_prod | samples | 6 | 6 … 6 | 5 |

## history_pagination page_2000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| offset_baseline | p50_us | 89780.959 | 89373.458 … 92639.125 | 5 |
| offset_baseline | p95_us | 95326.542 | 90384.625 … 110951.583 | 5 |
| offset_baseline | p999_us | 95326.542 | 90384.625 … 110951.583 | 5 |
| offset_baseline | p99_us | 95326.542 | 90384.625 … 110951.583 | 5 |
| offset_baseline | samples | 6 | 6 … 6 | 5 |
| cursor_before_id_prod | p50_us | 496 | 440.541 … 665.709 | 5 |
| cursor_before_id_prod | p95_us | 617.667 | 556.541 … 1330.792 | 5 |
| cursor_before_id_prod | p999_us | 617.667 | 556.541 … 1330.792 | 5 |
| cursor_before_id_prod | p99_us | 617.667 | 556.541 … 1330.792 | 5 |
| cursor_before_id_prod | samples | 6 | 6 … 6 | 5 |

## history_pagination page_3600 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| offset_baseline | p50_us | 161060.5 | 159880.708 … 161963.833 | 5 |
| offset_baseline | p95_us | 163630.417 | 162802.791 … 167929.125 | 5 |
| offset_baseline | p999_us | 163630.417 | 162802.791 … 167929.125 | 5 |
| offset_baseline | p99_us | 163630.417 | 162802.791 … 167929.125 | 5 |
| offset_baseline | samples | 6 | 6 … 6 | 5 |
| cursor_before_id_prod | p50_us | 455.417 | 418.625 … 462.333 | 5 |
| cursor_before_id_prod | p95_us | 610.083 | 580.084 … 669.291 | 5 |
| cursor_before_id_prod | p999_us | 610.083 | 580.084 … 669.291 | 5 |
| cursor_before_id_prod | p99_us | 610.083 | 580.084 … 669.291 | 5 |
| cursor_before_id_prod | samples | 6 | 6 … 6 | 5 |

## pg_archive_write batch_128 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| row_by_row_tx | alloc_bytes_per_batch | 72319.04 | 72296 … 72319.04 | 5 |
| row_by_row_tx | allocs_per_batch | 1676.01 | 1676 … 1676.01 | 5 |
| row_by_row_tx | batch_p50_us | 26198.458 | 25812.625 … 26996.459 | 5 |
| row_by_row_tx | batch_p95_us | 28993.167 | 27880.834 … 29894.708 | 5 |
| row_by_row_tx | batch_p999_us | 49409.417 | 31173.209 … 55055 | 5 |
| row_by_row_tx | batch_p99_us | 32574.625 | 29109.708 … 45664 | 5 |
| row_by_row_tx | batch_samples | 100 | 100 … 100 | 5 |
| row_by_row_tx | duplicate_rows | 0 | 0 … 0 | 5 |
| row_by_row_tx | rows_per_second | 4820.906235317528 | 4708.447837297678 … 4922.279576260656 | 5 |
| multivalue_insert | alloc_bytes_per_batch | 159332.8 | 159306.56 … 159346.8 | 5 |
| multivalue_insert | allocs_per_batch | 1328.31 | 1328.25 … 1328.33 | 5 |
| multivalue_insert | batch_p50_us | 1687.292 | 1650.25 … 1734.166 | 5 |
| multivalue_insert | batch_p95_us | 1899.167 | 1837.959 … 1991.583 | 5 |
| multivalue_insert | batch_p999_us | 15903.542 | 2126.291 … 27483 | 5 |
| multivalue_insert | batch_p99_us | 2134.208 | 2105.666 … 2175.375 | 5 |
| multivalue_insert | batch_samples | 100 | 100 … 100 | 5 |
| multivalue_insert | duplicate_rows | 0 | 0 … 0 | 5 |
| multivalue_insert | rows_per_second | 69944.27239276672 | 66462.54841108661 … 74208.88559455547 | 5 |
| unnest_prod | alloc_bytes_per_batch | 101256.8 | 101234.88 … 101257.6 | 5 |
| unnest_prod | allocs_per_batch | 827.94 | 827.94 … 827.95 | 5 |
| unnest_prod | batch_p50_us | 1670.958 | 1667.791 … 1691.459 | 5 |
| unnest_prod | batch_p95_us | 1794.25 | 1772.334 … 1854.25 | 5 |
| unnest_prod | batch_p999_us | 2621.75 | 1985.958 … 18020.834 | 5 |
| unnest_prod | batch_p99_us | 2094 | 1890.333 … 2436.583 | 5 |
| unnest_prod | batch_samples | 100 | 100 … 100 | 5 |
| unnest_prod | duplicate_rows | 0 | 0 … 0 | 5 |
| unnest_prod | rows_per_second | 75254.88550454212 | 69593.2377468763 … 77071.50394882173 | 5 |

## redis_lua_atomicity 0s · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| multi_cmd_baseline | extra_broadcasts | 35 | 35 … 35 | 5 |
| multi_cmd_baseline | p50_us | 715.875 | 711.667 … 720.958 | 5 |
| multi_cmd_baseline | p95_us | 752.167 | 738.833 … 863.084 | 5 |
| multi_cmd_baseline | p999_us | 815.25 | 748.75 … 1054.458 | 5 |
| multi_cmd_baseline | p99_us | 757.25 | 746.459 … 1052.375 | 5 |
| multi_cmd_baseline | qps | 1393.5582768651907 | 1353.368437274533 … 1402.8221274148268 | 5 |
| multi_cmd_baseline | race_duplicate_rate | 0.21875 | 0.21875 … 0.21875 | 5 |
| multi_cmd_baseline | samples | 100 | 100 … 100 | 5 |
| multi_cmd_baseline | stream_writes | 40 | 40 … 40 | 5 |
| lua_evalsha_prod | extra_broadcasts | 0 | 0 … 0 | 5 |
| lua_evalsha_prod | p50_us | 96.292 | 95.75 … 98.75 | 5 |
| lua_evalsha_prod | p95_us | 112.042 | 106.542 … 120.417 | 5 |
| lua_evalsha_prod | p999_us | 144.083 | 124.292 … 153.708 | 5 |
| lua_evalsha_prod | p99_us | 130.792 | 113.042 … 143.209 | 5 |
| lua_evalsha_prod | qps | 10193.37650703975 | 9831.512455297341 … 10376.89720811656 | 5 |
| lua_evalsha_prod | race_duplicate_rate | 0 | 0 … 0 | 5 |
| lua_evalsha_prod | samples | 100 | 100 … 100 | 5 |
| lua_evalsha_prod | stream_writes | 5 | 5 … 5 | 5 |

## redis_lua_atomicity 2ms · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| multi_cmd_baseline | extra_broadcasts | 35 | 35 … 35 | 5 |
| multi_cmd_baseline | p50_us | 25138.583 | 24659.334 … 26017.167 | 5 |
| multi_cmd_baseline | p95_us | 26399.167 | 26024.625 … 27612.375 | 5 |
| multi_cmd_baseline | p999_us | 35772.833 | 27292.541 … 39750.666 | 5 |
| multi_cmd_baseline | p99_us | 32680.667 | 26614.5 … 34237.042 | 5 |
| multi_cmd_baseline | qps | 39.77963276835316 | 38.23537682526541 … 41.21354944894394 | 5 |
| multi_cmd_baseline | race_duplicate_rate | 0.21875 | 0.21875 … 0.21875 | 5 |
| multi_cmd_baseline | samples | 100 | 100 … 100 | 5 |
| multi_cmd_baseline | stream_writes | 40 | 40 … 40 | 5 |
| lua_evalsha_prod | extra_broadcasts | 0 | 0 … 0 | 5 |
| lua_evalsha_prod | p50_us | 2881.875 | 2518.667 … 2999 | 5 |
| lua_evalsha_prod | p95_us | 3206.542 | 2807.125 … 3352.25 | 5 |
| lua_evalsha_prod | p999_us | 3370.375 | 3004 … 8779.417 | 5 |
| lua_evalsha_prod | p99_us | 3348.5 | 2954.958 … 4826.792 | 5 |
| lua_evalsha_prod | qps | 344.1684253443756 | 328.33767559909313 … 392.9593473731159 | 5 |
| lua_evalsha_prod | race_duplicate_rate | 0 | 0 … 0 | 5 |
| lua_evalsha_prod | samples | 100 | 100 … 100 | 5 |
| lua_evalsha_prod | stream_writes | 5 | 5 … 5 | 5 |

## sensitive_filter_scaling words_100 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| naive_contains | hit_us | 0.9095799999999999 | 0.86084 … 0.92125 | 5 |
| naive_contains | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| naive_contains | miss_us | 0.7 | 0.69958 … 0.72792 | 5 |
| merged_regexp | hit_us | 99.11917 | 97.49208 … 100.075 | 5 |
| merged_regexp | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| merged_regexp | miss_us | 99.04 | 97.42208000000001 … 100.61833 | 5 |
| ac_prod | hit_us | 1.49459 | 1.43166 … 1.59042 | 5 |
| ac_prod | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| ac_prod | miss_us | 1.49125 | 1.35583 … 1.5675 | 5 |

## sensitive_filter_scaling words_1000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| naive_contains | hit_us | 9.17667 | 8.9225 … 9.304170000000001 | 5 |
| naive_contains | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| naive_contains | miss_us | 6.717910000000001 | 6.515420000000001 … 6.77959 | 5 |
| merged_regexp | hit_us | 1499.655 | 1492.43292 … 1542.4308300000002 | 5 |
| merged_regexp | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| merged_regexp | miss_us | 1540.4379199999998 | 1475.22333 … 1610.2091599999999 | 5 |
| ac_prod | hit_us | 1.50458 | 1.39041 … 1.52083 | 5 |
| ac_prod | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| ac_prod | miss_us | 1.49208 | 1.32916 … 1.53166 | 5 |

## sensitive_filter_scaling words_5000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| naive_contains | hit_us | 46.29292 | 45.85875 … 47.03125 | 5 |
| naive_contains | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| naive_contains | miss_us | 34.645 | 34.2675 … 34.70917 | 5 |
| ac_prod | hit_us | 1.54584 | 1.38333 … 1.60625 | 5 |
| ac_prod | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| ac_prod | miss_us | 1.49166 | 1.39583 … 1.51875 | 5 |

## sensitive_filter_scaling words_20000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| naive_contains | hit_us | 187.66625 | 184.83916000000002 … 191.51958 | 5 |
| naive_contains | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| naive_contains | miss_us | 138.57875 | 138.44541999999998 … 138.6425 | 5 |
| ac_prod | hit_us | 1.47084 | 1.4283299999999999 … 1.49292 | 5 |
| ac_prod | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| ac_prod | miss_us | 1.46 | 1.31542 … 1.57542 | 5 |

## sensitive_filter_scaling reload_5000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| rwmutex_rebuild_baseline | reload_p50_us | 2 | 2 … 2.042 | 5 |
| rwmutex_rebuild_baseline | reload_p95_us | 9601.416 | 9383.083 … 11963.417 | 5 |
| rwmutex_rebuild_baseline | reload_p999_us | 29923.166 | 27871.333 … 34522.292 | 5 |
| rwmutex_rebuild_baseline | reload_p99_us | 25359.292 | 22838.5 … 27893.583 | 5 |
| rwmutex_rebuild_baseline | reload_samples | 160 | 160 … 161 | 5 |
| atomic_pointer_prod | reload_p50_us | 1.542 | 1.541 … 1.625 | 5 |
| atomic_pointer_prod | reload_p95_us | 3.583 | 3.417 … 4.334 | 5 |
| atomic_pointer_prod | reload_p999_us | 88.667 | 77.5 … 95.125 | 5 |
| atomic_pointer_prod | reload_p99_us | 10 | 8.5 … 10.291 | 5 |
| atomic_pointer_prod | reload_samples | 288523 | 251362 … 319654 | 5 |

## live_source_singleflight burst_120 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| direct_no_coalesce | cold_p50_us | 84353.166 | 84142.25 … 84802.625 | 5 |
| direct_no_coalesce | cold_p95_us | 85700.125 | 85304.25 … 86238.5 | 5 |
| direct_no_coalesce | cold_p999_us | 86633.166 | 85682.375 … 87596.375 | 5 |
| direct_no_coalesce | cold_p99_us | 86254.209 | 85621.5 … 87457.625 | 5 |
| direct_no_coalesce | cold_samples | 120 | 120 … 120 | 5 |
| direct_no_coalesce | cold_upstream_calls | 120 | 120 … 120 | 5 |
| direct_no_coalesce | warm_p50_us | 84856.958 | 83688.459 … 85092.625 | 5 |
| direct_no_coalesce | warm_p95_us | 86467.458 | 85109.25 … 87044 | 5 |
| direct_no_coalesce | warm_p999_us | 87274.125 | 85865.541 … 87923.75 | 5 |
| direct_no_coalesce | warm_p99_us | 87172.125 | 85669.959 … 87508.666 | 5 |
| direct_no_coalesce | warm_samples | 120 | 120 … 120 | 5 |
| direct_no_coalesce | warm_upstream_calls | 120 | 120 … 120 | 5 |
| prod_singleflight_cache | cold_p50_us | 82190.083 | 81224.917 … 82571.875 | 5 |
| prod_singleflight_cache | cold_p95_us | 82315.667 | 81456.833 … 82918.917 | 5 |
| prod_singleflight_cache | cold_p999_us | 82356.125 | 81503.542 … 82933.292 | 5 |
| prod_singleflight_cache | cold_p99_us | 82355.625 | 81500.042 … 82929.75 | 5 |
| prod_singleflight_cache | cold_samples | 120 | 120 … 120 | 5 |
| prod_singleflight_cache | cold_upstream_calls | 1 | 1 … 1 | 5 |
| prod_singleflight_cache | warm_p50_us | 0.5 | 0.375 … 0.708 | 5 |
| prod_singleflight_cache | warm_p95_us | 2.459 | 1.875 … 13.375 | 5 |
| prod_singleflight_cache | warm_p999_us | 16.542 | 4.959 … 66.542 | 5 |
| prod_singleflight_cache | warm_p99_us | 10.792 | 4.583 … 35.167 | 5 |
| prod_singleflight_cache | warm_samples | 120 | 120 … 120 | 5 |
| prod_singleflight_cache | warm_upstream_calls | 0 | 0 … 0 | 5 |

