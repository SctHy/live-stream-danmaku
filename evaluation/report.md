# Maku Evaluation

darwin/arm64 · go1.27.4 · CPUs 10 · profile full · repeats 5 (+ warmup) · 2026-11-25T12:00:15.613601Z · git_sha f29748d9ed7acba06cfe1b4942daceabb9f572f9 · dirty false

## Scorecard

| Experiment / load | Metric | Baseline → treatment | Medians | Change (%) | Δ 95% CI | Trade-off | Gate |
| --- | --- | --- | --- | --- | --- | --- | --- |
| fanout_sweep low | frames | legacy → adaptive_batch | 6400 → 6400 | 0 | 0 … 0 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | hot_p50_us | legacy → adaptive_batch | 1439 → 1436 | -0.20847810979847115 | -58 … 51 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | hot_p99_us | legacy → adaptive_batch | 2296 → 2589 | 12.76132404181185 | -73 … 804 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | process_cpu_seconds_total | legacy → adaptive_batch | 0.42448600000000003 → 0.421481 | -0.7079149842397658 | -0.015680000000000027 … 0.004691000000000001 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | frames | legacy → mixed_25pct_legacy | 6400 → 6400 | 0 | 0 … 0 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | hot_p50_us | legacy → mixed_25pct_legacy | 1439 → 1454 | 1.0423905489923557 | -298 … 54 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | hot_p99_us | legacy → mixed_25pct_legacy | 2296 → 2583 | 12.5 | -225 … 530 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | process_cpu_seconds_total | legacy → mixed_25pct_legacy | 0.42448600000000003 → 0.417848 | -1.5637735991293034 | -0.09900900000000001 … 0.0010580000000000034 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | frames | legacy → adaptive_batch | 76800 → 76800 | 0 | 0 … 0 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | hot_p50_us | legacy → adaptive_batch | 1348 → 1339 | -0.667655786350152 | -78 … 116 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | hot_p99_us | legacy → adaptive_batch | 2522 → 2790 | 10.626486915146716 | 17 … 554 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | process_cpu_seconds_total | legacy → adaptive_batch | 1.542633 → 1.5439690000000001 | 0.08660517439988968 | -0.06791699999999978 … 0.15653300000000003 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | frames | legacy → mixed_25pct_legacy | 76800 → 76800 | 0 | 0 … 0 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | hot_p50_us | legacy → mixed_25pct_legacy | 1348 → 1351 | 0.2225519287833766 | -57 … 126 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | hot_p99_us | legacy → mixed_25pct_legacy | 2522 → 2665 | 5.670103092783507 | -108 … 831 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | process_cpu_seconds_total | legacy → mixed_25pct_legacy | 1.542633 → 1.541423 | -0.07843732112563506 | -0.07255400000000001 … 0.15206699999999995 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | frames | legacy → adaptive_batch | 313600 → 99680 | -68.21428571428572 | -215264 … -212576 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | hot_p50_us | legacy → adaptive_batch | 800 → 4669 | 483.625 | 3821 … 4046 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | hot_p99_us | legacy → adaptive_batch | 4385 → 9984 | 127.68529076396806 | 4346 … 9184 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | process_cpu_seconds_total | legacy → adaptive_batch | 3.97621 → 1.702402 | -57.18530962901859 | -2.335573 … -2.2430199999999996 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | frames | legacy → mixed_25pct_legacy | 313600 → 148960 | -52.5 | -166152 … -164136 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | hot_p50_us | legacy → mixed_25pct_legacy | 800 → 3323 | 315.37499999999994 | 2461 … 2620 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | hot_p99_us | legacy → mixed_25pct_legacy | 4385 → 8440 | 92.47434435575828 | 2802 … 4794 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | process_cpu_seconds_total | legacy → mixed_25pct_legacy | 3.97621 → 2.3299030000000003 | -41.40392484300376 | -1.6972929999999997 … -1.5933480000000002 | Batching can increase hot-room latency while reducing frames. | PASS |
| isolation_stalled room_shards | cold_p99_us | single_shard → default_shards | 3741 → 3646 | -2.5394279604383896 | -1857 … 710 |  | PASS |
| isolation_stalled room_shards | danmaku_dropped_messages_total{reason="client_full"} | single_shard → default_shards | 0 → 0 | — | 0 … 0 |  | PASS |
| isolation_stalled room_shards | danmaku_kicked_slow_clients_total | single_shard → default_shards | 0 → 0 | — | 0 … 0 |  | PASS |
| isolation_stalled room_shards | rss_peak_bytes | single_shard → default_shards | 44662784 → 44253184 | -0.9170946441672823 | -1523712 … 1163264 |  | PASS |
| isolation_stalled slow_kick | cold_p99_us | kick_disabled → kick_enabled | 3413 → 3325 | -2.57837679460885 | -692 … 328 |  | PASS |
| isolation_stalled slow_kick | danmaku_dropped_messages_total{reason="client_full"} | kick_disabled → kick_enabled | 11992 → 32 | -99.73315543695797 | -11960 … -11960 |  | PASS |
| isolation_stalled slow_kick | danmaku_kicked_slow_clients_total | kick_disabled → kick_enabled | 0 → 4 | — | 4 … 4 |  | PASS |
| isolation_stalled slow_kick | rss_peak_bytes | kick_disabled → kick_enabled | 59555840 → 58589184 | -1.6231086657496507 | -2523136 … 1245184 |  | PASS |
| migration_strategies | connect_peak_qps | crash → fixed | 340 → 480 | 41.176470588235304 | 100 … 180 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| migration_strategies | drain_ms | crash → fixed | 453 → 408.03 | -9.927152317880806 | -48.97000000000003 … -29.970000000000027 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| migration_strategies | recent_peak_qps | crash → fixed | 340 → 480 | 41.176470588235304 | 30 … 180 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| migration_strategies | connect_peak_qps | crash → bounded | 340 → 80 | -76.47058823529412 | -300 … -220 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| migration_strategies | drain_ms | crash → bounded | 453 → 1907.667 | 321.1185430463576 | 1450.667 … 1469.667 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| migration_strategies | recent_peak_qps | crash → bounded | 340 → 80 | -76.47058823529412 | -300 … -220 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| history_pagination page_1 | p50_us | offset_baseline → cursor_before_id_prod | 344.75 → 355.583 | 3.1422770123277877 | -23.375 … 29.874000000000024 | Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps. | PASS |
| history_pagination page_200 | p50_us | offset_baseline → cursor_before_id_prod | 8623.5 → 354.375 | -95.89058966776831 | -8462.333 … -8218.667 | Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps. | PASS |
| history_pagination page_1000 | p50_us | offset_baseline → cursor_before_id_prod | 36112.333 → 386.916 | -98.92857656136478 | -35970.959 … -34906.667 | Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps. | PASS |
| history_pagination page_2000 | p50_us | offset_baseline → cursor_before_id_prod | 86538.125 → 398.333 | -99.53970229884227 | -86499.917 … -85442.042 | Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps. | PASS |
| history_pagination page_3600 | p50_us | offset_baseline → cursor_before_id_prod | 155210.083 → 383.959 | -99.7526198088561 | -155473.916 … -154020.666 | Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps. | PASS |
| pg_archive_write batch_128 | rows_per_second | row_by_row_tx → multivalue_insert | 6291.176410054454 → 93155.55362516547 | 1380.7334519548015 | 74674.16679858805 … 93040.56541666016 | UNNEST builds client arrays but uses five bind parameters per batch. | PASS |
| pg_archive_write batch_128 | rows_per_second | row_by_row_tx → unnest_prod | 6291.176410054454 → 103448.51943428337 | 1544.343008232191 | 75598.99407990646 … 98471.46589955977 | UNNEST builds client arrays but uses five bind parameters per batch. | PASS |
| redis_lua_atomicity 0s | p50_us | multi_cmd_baseline → lua_evalsha_prod | 718.25 → 93.583 | -86.97069265576053 | -626.8340000000001 … -621.2080000000001 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| redis_lua_atomicity 0s | qps | multi_cmd_baseline → lua_evalsha_prod | 1392.0621138115184 → 10544.02753593879 | 657.4394440682569 | 8781.438805567761 … 9252.803881113967 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| redis_lua_atomicity 0s | race_duplicate_rate | multi_cmd_baseline → lua_evalsha_prod | 0.21875 → 0 | -100 | -0.21875 … -0.21875 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| redis_lua_atomicity 2ms | p50_us | multi_cmd_baseline → lua_evalsha_prod | 26711.375 → 3030.5 | -88.65464619473913 | -23767.792 … -23227.458 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| redis_lua_atomicity 2ms | qps | multi_cmd_baseline → lua_evalsha_prod | 37.535917180752335 → 331.6536458063433 | 783.5634525973663 | 291.4770189729023 … 294.680475363806 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| redis_lua_atomicity 2ms | race_duplicate_rate | multi_cmd_baseline → lua_evalsha_prod | 0.21875 → 0 | -100 | -0.21875 … -0.21875 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| sensitive_filter_scaling words_100 | miss_us | naive_contains → merged_regexp | 0.635 → 95.99166 | 15016.7968503937 | 95.29374999999999 … 95.73458 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling words_100 | miss_us | naive_contains → ac_prod | 0.635 → 1.32875 | 109.25196850393704 | 0.60208 … 0.79125 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling words_1000 | miss_us | naive_contains → merged_regexp | 6.525410000000001 → 1486.92 | 22686.614174435017 | 1435.8520899999999 … 1510.61417 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling words_1000 | miss_us | naive_contains → ac_prod | 6.525410000000001 → 1.8525 | -71.61097923348878 | -5.43208 … -3.962490000000001 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling words_5000 | miss_us | naive_contains → ac_prod | 33.825 → 1.39458 | -95.8770731707317 | -32.85001 … -32.26541 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling words_20000 | miss_us | naive_contains → ac_prod | 137.0575 → 1.47125 | -98.9265454280138 | -135.76959 … -134.18125 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling reload_5000 | reload_p99_us | rwmutex_rebuild_baseline → atomic_pointer_prod | 25336.75 → 9.542 | -99.96233928976685 | -30283.333 … -18307.124 | Atomic reload temporarily retains old and new tries. | PASS |
| live_source_singleflight burst_120 | cold_upstream_calls | direct_no_coalesce → prod_singleflight_cache | 120 → 1 | -99.16666666666667 | -119 … -119 | Live results may remain cached for 15 seconds. | PASS |
| live_source_singleflight burst_120 | warm_upstream_calls | direct_no_coalesce → prod_singleflight_cache | 120 → 0 | -100 | -120 … -120 | Live results may remain cached for 15 seconds. | PASS |

## fanout_sweep low · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| legacy | achieved_rate | 99.92127242913818 | 99.91942542497472 … 99.9904071703121 | 5 |
| legacy | affected | 0 | 0 … 0 | 5 |
| legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| legacy | cold_gaps | 0 | 0 … 0 | 5 |
| legacy | cold_missing | 0 | 0 … 0 | 5 |
| legacy | cold_p50_us | 1418 | 1385 … 1477 | 5 |
| legacy | cold_p99_us | 2283 | 2057 … 2403 | 5 |
| legacy | cold_rate | 100 | 100 … 100 | 5 |
| legacy | cold_received | 200 | 200 … 200 | 5 |
| legacy | cold_samples | 200 | 200 … 200 | 5 |
| legacy | cold_unique | 200 | 200 … 200 | 5 |
| legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_count | 400 | 400 … 400 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_sum | 0.0016590850000000007 | 0.001554667000000001 … 0.001785409 | 5 |
| legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="single"} | 10400 | 10400 … 10400 | 5 |
| legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| legacy | duplicates | 0 | 0 … 0 | 5 |
| legacy | expected | 6400 | 6400 … 6400 | 5 |
| legacy | frames | 6400 | 6400 … 6400 | 5 |
| legacy | go_gc_duration_seconds_sum | 0.000498332 | 0.000450166 … 0.000685584 | 5 |
| legacy | go_goroutines | 1 | 0 … 1 | 5 |
| legacy | go_memstats_alloc_bytes_total | 14377912 | 11590544 … 16001608 | 5 |
| legacy | goroutines_peak | 176 | 176 … 177 | 5 |
| legacy | hot_p50_us | 1439 | 1420 … 1493 | 5 |
| legacy | hot_p99_us | 2296 | 2229 … 2344 | 5 |
| legacy | hot_samples | 1600 | 1600 … 1600 | 5 |
| legacy | missing | 0 | 0 … 0 | 5 |
| legacy | payload_bytes | 2194464 | 2192864 … 2194976 | 5 |
| legacy | process_cpu_seconds_total | 0.42448600000000003 | 0.41679 … 0.428723 | 5 |
| legacy | process_resident_memory_bytes | 10551296 | 10043392 … 11763712 | 5 |
| legacy | received | 6400 | 6400 … 6400 | 5 |
| legacy | rss_peak_bytes | 34930688 | 34553856 … 36405248 | 5 |
| legacy | seconds | 2 | 2 … 2 | 5 |
| legacy | target_rate | 100 | 100 … 100 | 5 |
| legacy | viewers | 32 | 32 … 32 | 5 |
| legacy | worst_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | achieved_rate | 99.95606720938407 | 99.89737208309707 … 99.99192980133364 | 5 |
| adaptive_batch | affected | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_p50_us | 1430 | 1345 … 1464 | 5 |
| adaptive_batch | cold_p99_us | 2396 | 1992 … 3667 | 5 |
| adaptive_batch | cold_rate | 100 | 100 … 100 | 5 |
| adaptive_batch | cold_received | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_samples | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_unique | 200 | 200 … 200 | 5 |
| adaptive_batch | cursor_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_count | 400 | 400 … 400 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_sum | 0.0017176660000000003 | 0.0014168840000000002 … 0.004170835000000001 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="single"} | 10400 | 10400 … 10400 | 5 |
| adaptive_batch | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| adaptive_batch | duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | expected | 6400 | 6400 … 6400 | 5 |
| adaptive_batch | frames | 6400 | 6400 … 6400 | 5 |
| adaptive_batch | go_gc_duration_seconds_sum | 0.00046233300000000005 | 0.000344959 … 0.00047258399999999996 | 5 |
| adaptive_batch | go_goroutines | 1 | 1 … 2 | 5 |
| adaptive_batch | go_memstats_alloc_bytes_total | 13318352 | 11147552 … 14212552 | 5 |
| adaptive_batch | goroutines_peak | 177 | 176 … 177 | 5 |
| adaptive_batch | hot_p50_us | 1436 | 1392 … 1489 | 5 |
| adaptive_batch | hot_p99_us | 2589 | 2223 … 3100 | 5 |
| adaptive_batch | hot_samples | 1600 | 1600 … 1600 | 5 |
| adaptive_batch | missing | 0 | 0 … 0 | 5 |
| adaptive_batch | payload_bytes | 2195168 | 2193376 … 2195328 | 5 |
| adaptive_batch | process_cpu_seconds_total | 0.421481 | 0.408806 … 0.422564 | 5 |
| adaptive_batch | process_resident_memory_bytes | 11419648 | 9601024 … 12058624 | 5 |
| adaptive_batch | received | 6400 | 6400 … 6400 | 5 |
| adaptive_batch | rss_peak_bytes | 35766272 | 33800192 … 36716544 | 5 |
| adaptive_batch | seconds | 2 | 2 … 2 | 5 |
| adaptive_batch | target_rate | 100 | 100 … 100 | 5 |
| adaptive_batch | viewers | 32 | 32 … 32 | 5 |
| adaptive_batch | worst_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | achieved_rate | 99.93306563210416 | 99.90294428962264 … 99.96526412000621 | 5 |
| mixed_25pct_legacy | affected | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_p50_us | 1381 | 1117 … 1405 | 5 |
| mixed_25pct_legacy | cold_p99_us | 2338 | 2074 … 2669 | 5 |
| mixed_25pct_legacy | cold_rate | 100 | 100 … 100 | 5 |
| mixed_25pct_legacy | cold_received | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_samples | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_unique | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_count | 600 | 600 … 600 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_sum | 0.007072700000000001 | 0.003559952000000003 … 0.007372781999999997 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="single"} | 10400 | 10400 … 10400 | 5 |
| mixed_25pct_legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | expected | 6400 | 6400 … 6400 | 5 |
| mixed_25pct_legacy | frames | 6400 | 6400 … 6400 | 5 |
| mixed_25pct_legacy | go_gc_duration_seconds_sum | 0.00043866699999999995 | 0.000410751 … 0.000571708 | 5 |
| mixed_25pct_legacy | go_goroutines | 1 | 1 … 2 | 5 |
| mixed_25pct_legacy | go_memstats_alloc_bytes_total | 12259816 | 11132712 … 12295184 | 5 |
| mixed_25pct_legacy | goroutines_peak | 177 | 176 … 177 | 5 |
| mixed_25pct_legacy | hot_p50_us | 1454 | 1141 … 1493 | 5 |
| mixed_25pct_legacy | hot_p99_us | 2583 | 2071 … 2764 | 5 |
| mixed_25pct_legacy | hot_samples | 1600 | 1600 … 1600 | 5 |
| mixed_25pct_legacy | missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | payload_bytes | 2194944 | 2193984 … 2195168 | 5 |
| mixed_25pct_legacy | process_cpu_seconds_total | 0.417848 | 0.325477 … 0.420329 | 5 |
| mixed_25pct_legacy | process_resident_memory_bytes | 9863168 | 9191424 … 10534912 | 5 |
| mixed_25pct_legacy | received | 6400 | 6400 … 6400 | 5 |
| mixed_25pct_legacy | rss_peak_bytes | 34373632 | 33505280 … 34848768 | 5 |
| mixed_25pct_legacy | seconds | 2 | 2 … 2 | 5 |
| mixed_25pct_legacy | target_rate | 100 | 100 … 100 | 5 |
| mixed_25pct_legacy | viewers | 32 | 32 … 32 | 5 |
| mixed_25pct_legacy | worst_missing | 0 | 0 … 0 | 5 |

## fanout_sweep medium · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| legacy | achieved_rate | 399.949856286768 | 399.9376597172916 … 399.9688524256174 | 5 |
| legacy | affected | 0 | 0 … 0 | 5 |
| legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| legacy | cold_gaps | 0 | 0 … 0 | 5 |
| legacy | cold_missing | 0 | 0 … 0 | 5 |
| legacy | cold_p50_us | 1351 | 1174 … 1524 | 5 |
| legacy | cold_p99_us | 2646 | 2384 … 3113 | 5 |
| legacy | cold_rate | 100 | 100 … 100 | 5 |
| legacy | cold_received | 200 | 200 … 200 | 5 |
| legacy | cold_samples | 200 | 200 … 200 | 5 |
| legacy | cold_unique | 200 | 200 … 200 | 5 |
| legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_count | 1000 | 1000 … 1000 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_sum | 0.0029522990000000007 | 0.002362567000000003 … 0.0045592459999999994 | 5 |
| legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="single"} | 80800 | 80800 … 80800 | 5 |
| legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| legacy | duplicates | 0 | 0 … 0 | 5 |
| legacy | expected | 76800 | 76800 … 76800 | 5 |
| legacy | frames | 76800 | 76800 … 76800 | 5 |
| legacy | go_gc_duration_seconds_sum | 0.000443044 | 0.000390041 … 0.000529208 | 5 |
| legacy | go_goroutines | 1 | 0 … 1 | 5 |
| legacy | go_memstats_alloc_bytes_total | 18650064 | 17739696 … 19416192 | 5 |
| legacy | goroutines_peak | 305 | 304 … 305 | 5 |
| legacy | hot_p50_us | 1348 | 1225 … 1380 | 5 |
| legacy | hot_p99_us | 2522 | 2450 … 2666 | 5 |
| legacy | hot_samples | 6400 | 6400 … 6400 | 5 |
| legacy | missing | 0 | 0 … 0 | 5 |
| legacy | payload_bytes | 26375040 | 26372640 … 26379648 | 5 |
| legacy | process_cpu_seconds_total | 1.542633 | 1.389356 … 1.5911899999999999 | 5 |
| legacy | process_resident_memory_bytes | 14548992 | 14286848 … 15007744 | 5 |
| legacy | received | 76800 | 76800 … 76800 | 5 |
| legacy | rss_peak_bytes | 38912000 | 38649856 … 39223296 | 5 |
| legacy | seconds | 2 | 2 … 2 | 5 |
| legacy | target_rate | 400 | 400 … 400 | 5 |
| legacy | viewers | 96 | 96 … 96 | 5 |
| legacy | worst_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | achieved_rate | 399.9409337233508 | 399.9356435560459 … 399.9481483223626 | 5 |
| adaptive_batch | affected | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_p50_us | 1404 | 1219 … 1469 | 5 |
| adaptive_batch | cold_p99_us | 2773 | 2411 … 3905 | 5 |
| adaptive_batch | cold_rate | 100 | 100 … 100 | 5 |
| adaptive_batch | cold_received | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_samples | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_unique | 200 | 200 … 200 | 5 |
| adaptive_batch | cursor_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_count | 1000 | 1000 … 1000 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_sum | 0.0029672540000000008 | 0.0026951390000000018 … 0.0030945380000000017 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="single"} | 80800 | 80800 … 80800 | 5 |
| adaptive_batch | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| adaptive_batch | duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | expected | 76800 | 76800 … 76800 | 5 |
| adaptive_batch | frames | 76800 | 76800 … 76800 | 5 |
| adaptive_batch | go_gc_duration_seconds_sum | 0.000503502 | 0.000414083 … 0.001232248 | 5 |
| adaptive_batch | go_goroutines | 1 | 0 … 1 | 5 |
| adaptive_batch | go_memstats_alloc_bytes_total | 17506240 | 17276688 … 19504672 | 5 |
| adaptive_batch | goroutines_peak | 305 | 304 … 305 | 5 |
| adaptive_batch | hot_p50_us | 1339 | 1270 … 1399 | 5 |
| adaptive_batch | hot_p99_us | 2790 | 2539 … 3076 | 5 |
| adaptive_batch | hot_samples | 6400 | 6400 … 6400 | 5 |
| adaptive_batch | missing | 0 | 0 … 0 | 5 |
| adaptive_batch | payload_bytes | 26378016 | 26376480 … 26385408 | 5 |
| adaptive_batch | process_cpu_seconds_total | 1.5439690000000001 | 1.504554 … 1.623021 | 5 |
| adaptive_batch | process_resident_memory_bytes | 14860288 | 13991936 … 15253504 | 5 |
| adaptive_batch | received | 76800 | 76800 … 76800 | 5 |
| adaptive_batch | rss_peak_bytes | 38928384 | 38387712 … 39534592 | 5 |
| adaptive_batch | seconds | 2 | 2 … 2 | 5 |
| adaptive_batch | target_rate | 400 | 400 … 400 | 5 |
| adaptive_batch | viewers | 96 | 96 … 96 | 5 |
| adaptive_batch | worst_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | achieved_rate | 399.9549966638204 | 399.93112026308825 … 399.9556717130371 | 5 |
| mixed_25pct_legacy | affected | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_p50_us | 1376 | 1227 … 1565 | 5 |
| mixed_25pct_legacy | cold_p99_us | 2738 | 2653 … 3774 | 5 |
| mixed_25pct_legacy | cold_rate | 100 | 100 … 100 | 5 |
| mixed_25pct_legacy | cold_received | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_samples | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_unique | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_count | 1800 | 1800 … 1800 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_sum | 0.01248060499999999 | 0.01205448199999998 … 0.014814963000000004 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="single"} | 80800 | 80800 … 80800 | 5 |
| mixed_25pct_legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | expected | 76800 | 76800 … 76800 | 5 |
| mixed_25pct_legacy | frames | 76800 | 76800 … 76800 | 5 |
| mixed_25pct_legacy | go_gc_duration_seconds_sum | 0.000587918 | 0.00033096 … 0.001442211 | 5 |
| mixed_25pct_legacy | go_goroutines | 1 | 0 … 1 | 5 |
| mixed_25pct_legacy | go_memstats_alloc_bytes_total | 18406672 | 17306504 … 19808176 | 5 |
| mixed_25pct_legacy | goroutines_peak | 305 | 304 … 305 | 5 |
| mixed_25pct_legacy | hot_p50_us | 1351 | 1291 … 1362 | 5 |
| mixed_25pct_legacy | hot_p99_us | 2665 | 2505 … 3353 | 5 |
| mixed_25pct_legacy | hot_samples | 6400 | 6400 … 6400 | 5 |
| mixed_25pct_legacy | missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | payload_bytes | 26375904 | 26374464 … 26381280 | 5 |
| mixed_25pct_legacy | process_cpu_seconds_total | 1.541423 | 1.470079 … 1.6063239999999999 | 5 |
| mixed_25pct_legacy | process_resident_memory_bytes | 14729216 | 13615104 … 15450112 | 5 |
| mixed_25pct_legacy | received | 76800 | 76800 … 76800 | 5 |
| mixed_25pct_legacy | rss_peak_bytes | 39174144 | 37978112 … 39878656 | 5 |
| mixed_25pct_legacy | seconds | 2 | 2 … 2 | 5 |
| mixed_25pct_legacy | target_rate | 400 | 400 … 400 | 5 |
| mixed_25pct_legacy | viewers | 96 | 96 … 96 | 5 |
| mixed_25pct_legacy | worst_missing | 0 | 0 … 0 | 5 |

## fanout_sweep high · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| legacy | achieved_rate | 699.9596835721543 | 699.9567089274755 … 699.9620020627451 | 5 |
| legacy | affected | 0 | 0 … 0 | 5 |
| legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| legacy | cold_gaps | 0 | 0 … 0 | 5 |
| legacy | cold_missing | 0 | 0 … 0 | 5 |
| legacy | cold_p50_us | 480 | 388 … 509 | 5 |
| legacy | cold_p99_us | 2491 | 2049 … 6379 | 5 |
| legacy | cold_rate | 100 | 100 … 100 | 5 |
| legacy | cold_received | 200 | 200 … 200 | 5 |
| legacy | cold_samples | 200 | 200 … 200 | 5 |
| legacy | cold_unique | 200 | 200 … 200 | 5 |
| legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_count | 1600 | 1600 … 1600 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_sum | 0.0017132600000000016 | 0.001393843999999994 … 0.001977415000000018 | 5 |
| legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="single"} | 317600 | 317600 … 317600 | 5 |
| legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| legacy | duplicates | 0 | 0 … 0 | 5 |
| legacy | expected | 313600 | 313600 … 313600 | 5 |
| legacy | frames | 313600 | 313600 … 313600 | 5 |
| legacy | go_gc_duration_seconds_sum | 0.00028816700000000003 | 0.000247625 … 0.000481291 | 5 |
| legacy | go_goroutines | 1 | 1 … 1 | 5 |
| legacy | go_memstats_alloc_bytes_total | 26376056 | 25289560 … 26832264 | 5 |
| legacy | goroutines_peak | 561 | 560 … 562 | 5 |
| legacy | hot_p50_us | 800 | 754 … 841 | 5 |
| legacy | hot_p99_us | 4385 | 3646 … 5638 | 5 |
| legacy | hot_samples | 11200 | 11200 … 11200 | 5 |
| legacy | missing | 0 | 0 … 0 | 5 |
| legacy | payload_bytes | 107826880 | 107826656 … 107834272 | 5 |
| legacy | process_cpu_seconds_total | 3.97621 | 3.964443 … 4.027196 | 5 |
| legacy | process_resident_memory_bytes | 22429696 | 22151168 … 23330816 | 5 |
| legacy | received | 313600 | 313600 … 313600 | 5 |
| legacy | rss_peak_bytes | 46776320 | 46333952 … 47890432 | 5 |
| legacy | seconds | 2 | 2 … 2 | 5 |
| legacy | target_rate | 700 | 700 … 700 | 5 |
| legacy | viewers | 224 | 224 … 224 | 5 |
| legacy | worst_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | achieved_rate | 699.9280280007163 | 699.9091908319641 … 699.9787131473485 | 5 |
| adaptive_batch | affected | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_p50_us | 883 | 809 … 971 | 5 |
| adaptive_batch | cold_p99_us | 4282 | 3508 … 10723 | 5 |
| adaptive_batch | cold_rate | 100 | 100 … 100 | 5 |
| adaptive_batch | cold_received | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_samples | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_unique | 200 | 200 … 200 | 5 |
| adaptive_batch | cursor_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_count | 645 | 639 … 651 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_sum | 0.001741927999999999 | 0.0015844339999999991 … 0.0026373789999999996 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="batch"} | 63392 | 62272 … 64736 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="single"} | 40288 | 37600 … 42752 | 5 |
| adaptive_batch | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| adaptive_batch | duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | expected | 313600 | 313600 … 313600 | 5 |
| adaptive_batch | frames | 99680 | 98336 … 101024 | 5 |
| adaptive_batch | go_gc_duration_seconds_sum | 0.000411708 | 0.00025441699999999995 … 0.000531626 | 5 |
| adaptive_batch | go_goroutines | 1 | 1 … 1 | 5 |
| adaptive_batch | go_memstats_alloc_bytes_total | 25386712 | 21701016 … 27794184 | 5 |
| adaptive_batch | goroutines_peak | 561 | 560 … 561 | 5 |
| adaptive_batch | hot_p50_us | 4669 | 4621 … 4846 | 5 |
| adaptive_batch | hot_p99_us | 9984 | 9813 … 13569 | 5 |
| adaptive_batch | hot_samples | 11200 | 11200 … 11200 | 5 |
| adaptive_batch | missing | 0 | 0 … 0 | 5 |
| adaptive_batch | payload_bytes | 108168928 | 108149664 … 108171840 | 5 |
| adaptive_batch | process_cpu_seconds_total | 1.702402 | 1.679879 … 1.727789 | 5 |
| adaptive_batch | process_resident_memory_bytes | 22822912 | 21331968 … 23183360 | 5 |
| adaptive_batch | received | 313600 | 313600 … 313600 | 5 |
| adaptive_batch | rss_peak_bytes | 47087616 | 45858816 … 47546368 | 5 |
| adaptive_batch | seconds | 2 | 2 … 2 | 5 |
| adaptive_batch | target_rate | 700 | 700 … 700 | 5 |
| adaptive_batch | viewers | 224 | 224 … 224 | 5 |
| adaptive_batch | worst_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | achieved_rate | 699.9891836171371 | 699.9323002981328 … 699.9922897849258 | 5 |
| mixed_25pct_legacy | affected | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_p50_us | 829 | 747 … 878 | 5 |
| mixed_25pct_legacy | cold_p99_us | 3283 | 3096 … 4537 | 5 |
| mixed_25pct_legacy | cold_rate | 100 | 100 … 100 | 5 |
| mixed_25pct_legacy | cold_received | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_samples | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_unique | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_count | 2020 | 2011 … 2023 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_sum | 0.009924371999999957 | 0.008273315000000007 … 0.012437830999999925 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="batch"} | 51912 | 51576 … 52920 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="single"} | 101216 | 98528 … 101720 | 5 |
| mixed_25pct_legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | expected | 313600 | 313600 … 313600 | 5 |
| mixed_25pct_legacy | frames | 148960 | 147448 … 149464 | 5 |
| mixed_25pct_legacy | go_gc_duration_seconds_sum | 0.000430917 | 0.000303375 … 0.000724958 | 5 |
| mixed_25pct_legacy | go_goroutines | 1 | 0 … 2 | 5 |
| mixed_25pct_legacy | go_memstats_alloc_bytes_total | 26887688 | 24987608 … 28088192 | 5 |
| mixed_25pct_legacy | goroutines_peak | 560 | 560 … 561 | 5 |
| mixed_25pct_legacy | hot_p50_us | 3323 | 3261 … 3420 | 5 |
| mixed_25pct_legacy | hot_p99_us | 8440 | 8334 … 8512 | 5 |
| mixed_25pct_legacy | hot_samples | 11200 | 11200 … 11200 | 5 |
| mixed_25pct_legacy | missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | payload_bytes | 108085600 | 108077592 … 108104472 | 5 |
| mixed_25pct_legacy | process_cpu_seconds_total | 2.3299030000000003 | 2.291502 … 2.382862 | 5 |
| mixed_25pct_legacy | process_resident_memory_bytes | 22691840 | 22315008 … 24100864 | 5 |
| mixed_25pct_legacy | received | 313600 | 313600 … 313600 | 5 |
| mixed_25pct_legacy | rss_peak_bytes | 46972928 | 46481408 … 48201728 | 5 |
| mixed_25pct_legacy | seconds | 2 | 2 … 2 | 5 |
| mixed_25pct_legacy | target_rate | 700 | 700 … 700 | 5 |
| mixed_25pct_legacy | viewers | 224 | 224 … 224 | 5 |
| mixed_25pct_legacy | worst_missing | 0 | 0 … 0 | 5 |

## isolation_stalled room_shards · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| single_shard | achieved_rate | 599.9808631103871 | 599.9172490144504 … 599.9922625997804 | 5 |
| single_shard | affected | 0 | 0 … 0 | 5 |
| single_shard | cold_duplicates | 0 | 0 … 0 | 5 |
| single_shard | cold_gaps | 0 | 0 … 0 | 5 |
| single_shard | cold_missing | 0 | 0 … 0 | 5 |
| single_shard | cold_p50_us | 963 | 938 … 1016 | 5 |
| single_shard | cold_p99_us | 3741 | 3287 … 5503 | 5 |
| single_shard | cold_rate | 100 | 100 … 100 | 5 |
| single_shard | cold_received | 200 | 200 … 200 | 5 |
| single_shard | cold_samples | 200 | 200 … 200 | 5 |
| single_shard | cold_unique | 200 | 200 … 200 | 5 |
| single_shard | cursor_gaps | 0 | 0 … 0 | 5 |
| single_shard | danmaku_delivery_queue_wait_seconds_count | 667 | 662 … 685 | 5 |
| single_shard | danmaku_delivery_queue_wait_seconds_sum | 0.002852194000000004 | 0.002567325000000004 … 0.0030809899999999966 | 5 |
| single_shard | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| single_shard | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| single_shard | danmaku_fanout_frames_total{mode="batch"} | 43040 | 41280 … 44480 | 5 |
| single_shard | danmaku_fanout_frames_total{mode="single"} | 35203 | 34240 … 39683 | 5 |
| single_shard | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| single_shard | duplicates | 0 | 0 … 0 | 5 |
| single_shard | expected | 192000 | 192000 … 192000 | 5 |
| single_shard | frames | 74720 | 73760 … 76960 | 5 |
| single_shard | go_gc_duration_seconds_sum | 0.000351417 | 0.000301668 … 0.0006522090000000001 | 5 |
| single_shard | go_goroutines | 14 | 10 … 15 | 5 |
| single_shard | go_memstats_alloc_bytes_total | 24591992 | 23761504 … 25967032 | 5 |
| single_shard | goroutines_peak | 466 | 465 … 466 | 5 |
| single_shard | hot_p50_us | 3996 | 3894 … 4178 | 5 |
| single_shard | hot_p99_us | 8992 | 8657 … 12006 | 5 |
| single_shard | hot_samples | 9600 | 9600 … 9600 | 5 |
| single_shard | missing | 0 | 0 … 0 | 5 |
| single_shard | payload_bytes | 66190560 | 66163520 … 66203680 | 5 |
| single_shard | process_cpu_seconds_total | 1.359772 | 1.324122 … 1.414078 | 5 |
| single_shard | process_resident_memory_bytes | 20725760 | 20267008 … 21561344 | 5 |
| single_shard | received | 192000 | 192000 … 192000 | 5 |
| single_shard | rss_peak_bytes | 44662784 | 44302336 … 45563904 | 5 |
| single_shard | seconds | 2 | 2 … 2 | 5 |
| single_shard | target_rate | 600 | 600 … 600 | 5 |
| single_shard | viewers | 160 | 160 … 160 | 5 |
| single_shard | worst_missing | 0 | 0 … 0 | 5 |
| default_shards | achieved_rate | 599.9703015700674 | 599.9423057482447 … 599.9919500080055 | 5 |
| default_shards | affected | 0 | 0 … 0 | 5 |
| default_shards | cold_duplicates | 0 | 0 … 0 | 5 |
| default_shards | cold_gaps | 0 | 0 … 0 | 5 |
| default_shards | cold_missing | 0 | 0 … 0 | 5 |
| default_shards | cold_p50_us | 978 | 957 … 1042 | 5 |
| default_shards | cold_p99_us | 3646 | 3065 … 4165 | 5 |
| default_shards | cold_rate | 100 | 100 … 100 | 5 |
| default_shards | cold_received | 200 | 200 … 200 | 5 |
| default_shards | cold_samples | 200 | 200 … 200 | 5 |
| default_shards | cold_unique | 200 | 200 … 200 | 5 |
| default_shards | cursor_gaps | 0 | 0 … 0 | 5 |
| default_shards | danmaku_delivery_queue_wait_seconds_count | 673 | 671 … 683 | 5 |
| default_shards | danmaku_delivery_queue_wait_seconds_sum | 0.0018594640000000032 | 0.0016948380000000008 … 0.004012291000000007 | 5 |
| default_shards | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| default_shards | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| default_shards | danmaku_fanout_frames_total{mode="batch"} | 42880 | 41600 … 44000 | 5 |
| default_shards | danmaku_fanout_frames_total{mode="single"} | 37122 | 35520 … 38080 | 5 |
| default_shards | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| default_shards | duplicates | 0 | 0 … 0 | 5 |
| default_shards | expected | 192000 | 192000 … 192000 | 5 |
| default_shards | frames | 75680 | 75040 … 76800 | 5 |
| default_shards | go_gc_duration_seconds_sum | 0.000359582 | 0.00029650300000000004 … 0.00046720700000000003 | 5 |
| default_shards | go_goroutines | 16 | 9 … 20 | 5 |
| default_shards | go_memstats_alloc_bytes_total | 25137640 | 24864200 … 26088768 | 5 |
| default_shards | goroutines_peak | 497 | 496 … 497 | 5 |
| default_shards | hot_p50_us | 3983 | 3953 … 4010 | 5 |
| default_shards | hot_p99_us | 8816 | 8278 … 9042 | 5 |
| default_shards | hot_samples | 9600 | 9600 … 9600 | 5 |
| default_shards | missing | 0 | 0 … 0 | 5 |
| default_shards | payload_bytes | 66190080 | 66186080 … 66204480 | 5 |
| default_shards | process_cpu_seconds_total | 1.378371 | 1.350474 … 1.41802 | 5 |
| default_shards | process_resident_memory_bytes | 19841024 | 19283968 … 21495808 | 5 |
| default_shards | received | 192000 | 192000 … 192000 | 5 |
| default_shards | rss_peak_bytes | 44253184 | 43466752 … 45662208 | 5 |
| default_shards | seconds | 2 | 2 … 2 | 5 |
| default_shards | target_rate | 600 | 600 … 600 | 5 |
| default_shards | viewers | 160 | 160 … 160 | 5 |
| default_shards | worst_missing | 0 | 0 … 0 | 5 |

## isolation_stalled slow_kick · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| kick_disabled | achieved_rate | 599.989791807015 | 599.969939039511 … 599.996837450003 | 5 |
| kick_disabled | affected | 0 | 0 … 0 | 5 |
| kick_disabled | cold_duplicates | 0 | 0 … 0 | 5 |
| kick_disabled | cold_gaps | 0 | 0 … 0 | 5 |
| kick_disabled | cold_missing | 0 | 0 … 0 | 5 |
| kick_disabled | cold_p50_us | 541 | 515 … 581 | 5 |
| kick_disabled | cold_p99_us | 3413 | 3350 … 3950 | 5 |
| kick_disabled | cold_rate | 100 | 100 … 100 | 5 |
| kick_disabled | cold_received | 600 | 600 … 600 | 5 |
| kick_disabled | cold_samples | 600 | 600 … 600 | 5 |
| kick_disabled | cold_unique | 600 | 600 … 600 | 5 |
| kick_disabled | cursor_gaps | 0 | 0 … 0 | 5 |
| kick_disabled | danmaku_delivery_queue_wait_seconds_count | 5452 | 5443 … 5460 | 5 |
| kick_disabled | danmaku_delivery_queue_wait_seconds_sum | 0.017442495999999835 | 0.016904362999999787 … 0.019125788999999827 | 5 |
| kick_disabled | danmaku_dropped_messages_total{reason="client_full"} | 11992 | 11992 … 11992 | 5 |
| kick_disabled | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| kick_disabled | danmaku_fanout_frames_total{mode="batch"} | 151680 | 149760 … 154400 | 5 |
| kick_disabled | danmaku_fanout_frames_total{mode="single"} | 62702 | 58215 … 64293 | 5 |
| kick_disabled | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| kick_disabled | duplicates | 0 | 0 … 0 | 5 |
| kick_disabled | expected | 576000 | 576000 … 576000 | 5 |
| kick_disabled | frames | 200320 | 198880 … 201600 | 5 |
| kick_disabled | go_gc_duration_seconds_sum | 0.0014812879999999999 | 0.001041794 … 0.002130211 | 5 |
| kick_disabled | go_goroutines | 2 | 1 … 11 | 5 |
| kick_disabled | go_memstats_alloc_bytes_total | 129227624 | 127521736 … 132414464 | 5 |
| kick_disabled | goroutines_peak | 505 | 505 … 506 | 5 |
| kick_disabled | hot_p50_us | 4578 | 4554 … 4622 | 5 |
| kick_disabled | hot_p99_us | 9403 | 9258 … 9776 | 5 |
| kick_disabled | hot_samples | 28800 | 28800 … 28800 | 5 |
| kick_disabled | missing | 0 | 0 … 0 | 5 |
| kick_disabled | payload_bytes | 890780320 | 890756320 … 890796800 | 5 |
| kick_disabled | process_cpu_seconds_total | 3.736409 | 3.717339 … 3.782445 | 5 |
| kick_disabled | process_resident_memory_bytes | 35192832 | 33259520 … 35553280 | 5 |
| kick_disabled | received | 576000 | 576000 … 576000 | 5 |
| kick_disabled | rss_peak_bytes | 59555840 | 57344000 … 59899904 | 5 |
| kick_disabled | seconds | 6 | 6 … 6 | 5 |
| kick_disabled | target_rate | 600 | 600 … 600 | 5 |
| kick_disabled | viewers | 160 | 160 … 160 | 5 |
| kick_disabled | worst_missing | 0 | 0 … 0 | 5 |
| kick_enabled | achieved_rate | 599.9784008109029 | 599.9617732689601 … 599.9968042503548 | 5 |
| kick_enabled | affected | 0 | 0 … 0 | 5 |
| kick_enabled | cold_duplicates | 0 | 0 … 0 | 5 |
| kick_enabled | cold_gaps | 0 | 0 … 0 | 5 |
| kick_enabled | cold_missing | 0 | 0 … 0 | 5 |
| kick_enabled | cold_p50_us | 550 | 532 … 581 | 5 |
| kick_enabled | cold_p99_us | 3325 | 3024 … 3741 | 5 |
| kick_enabled | cold_rate | 100 | 100 … 100 | 5 |
| kick_enabled | cold_received | 600 | 600 … 600 | 5 |
| kick_enabled | cold_samples | 600 | 600 … 600 | 5 |
| kick_enabled | cold_unique | 600 | 600 … 600 | 5 |
| kick_enabled | cursor_gaps | 0 | 0 … 0 | 5 |
| kick_enabled | danmaku_delivery_queue_wait_seconds_count | 2472 | 2462 … 2490 | 5 |
| kick_enabled | danmaku_delivery_queue_wait_seconds_sum | 0.0061821729999999905 | 0.0058410610000000016 … 0.006340550000000011 | 5 |
| kick_enabled | danmaku_dropped_messages_total{reason="client_full"} | 32 | 32 … 32 | 5 |
| kick_enabled | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| kick_enabled | danmaku_fanout_frames_total{mode="batch"} | 150720 | 148320 … 154400 | 5 |
| kick_enabled | danmaku_fanout_frames_total{mode="single"} | 64937 | 59490 … 67011 | 5 |
| kick_enabled | danmaku_kicked_slow_clients_total | 4 | 4 … 4 | 5 |
| kick_enabled | duplicates | 0 | 0 … 0 | 5 |
| kick_enabled | expected | 576000 | 576000 … 576000 | 5 |
| kick_enabled | frames | 201600 | 200160 … 204320 | 5 |
| kick_enabled | go_gc_duration_seconds_sum | 0.001352292 | 0.001252544 … 0.001605541 | 5 |
| kick_enabled | go_goroutines | 5 | 1 … 17 | 5 |
| kick_enabled | go_memstats_alloc_bytes_total | 126866648 | 126493936 … 128963160 | 5 |
| kick_enabled | goroutines_peak | 504 | 504 … 505 | 5 |
| kick_enabled | hot_p50_us | 4496 | 4456 … 4539 | 5 |
| kick_enabled | hot_p99_us | 9321 | 9218 … 9430 | 5 |
| kick_enabled | hot_samples | 28800 | 28800 … 28800 | 5 |
| kick_enabled | missing | 0 | 0 … 0 | 5 |
| kick_enabled | payload_bytes | 890796960 | 890772800 … 890815360 | 5 |
| kick_enabled | process_cpu_seconds_total | 3.630279 | 3.552854 … 3.6776950000000004 | 5 |
| kick_enabled | process_resident_memory_bytes | 34111488 | 32866304 … 35078144 | 5 |
| kick_enabled | received | 576000 | 576000 … 576000 | 5 |
| kick_enabled | rss_peak_bytes | 58589184 | 57032704 … 59244544 | 5 |
| kick_enabled | seconds | 6 | 6 … 6 | 5 |
| kick_enabled | target_rate | 600 | 600 … 600 | 5 |
| kick_enabled | viewers | 160 | 160 … 160 | 5 |
| kick_enabled | worst_missing | 0 | 0 … 0 | 5 |

## migration_strategies · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| crash | complete_viewers | 144 | 144 … 144 | 5 |
| crash | connect_peak_qps | 340 | 300 … 380 | 5 |
| crash | danmaku_delivery_queue_wait_seconds_count | 24 | 24 … 24 | 5 |
| crash | danmaku_delivery_queue_wait_seconds_sum | 0.00018958100000000003 | 0.000164669 … 0.000214833 | 5 |
| crash | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| crash | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| crash | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| crash | danmaku_fanout_frames_total{mode="single"} | 3213 | 3205 … 3219 | 5 |
| crash | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| crash | drain_ms | 453 | 438 … 457 | 5 |
| crash | events | 24 | 24 … 24 | 5 |
| crash | go_gc_duration_seconds_sum | 0.000561503 | 0.000353083 … 0.001336417 | 5 |
| crash | go_goroutines | 6 | 4 … 6 | 5 |
| crash | go_memstats_alloc_bytes_total | 7474152 | 7056416 … 7899104 | 5 |
| crash | invited | 0 | 0 … 0 | 5 |
| crash | last_handoff_ms | 453 | 438 … 457 | 5 |
| crash | migrating_delivery_p50_us | 2652 | 2489 … 2696 | 5 |
| crash | migrating_delivery_p95_us | 167712 | 157878 … 178945 | 5 |
| crash | migrating_delivery_p99_us | 384459 | 358668 … 404249 | 5 |
| crash | migrating_delivery_samples | 1584 | 1584 … 1584 | 5 |
| crash | moved | 72 | 72 … 72 | 5 |
| crash | peak_attempts_per_sec | 72 | 72 … 72 | 5 |
| crash | process_cpu_seconds_total | 0.222924 | 0.176601 … 0.23213 | 5 |
| crash | process_resident_memory_bytes | 10240000 | 9682944 … 10616832 | 5 |
| crash | recent_peak_qps | 340 | 300 … 380 | 5 |
| crash | recovery_p50_ms | 304 | 278 … 328 | 5 |
| crash | recovery_p95_ms | 436 | 413 … 438 | 5 |
| crash | recovery_p99_ms | 447 | 432 … 452 | 5 |
| crash | recovery_samples | 72 | 72 … 72 | 5 |
| crash | resident_delivery_p50_us | 2592 | 2421 … 2634 | 5 |
| crash | resident_delivery_p95_us | 3554 | 3183 … 3994 | 5 |
| crash | resident_delivery_p99_ms | 4 | 3 … 4 | 5 |
| crash | resident_delivery_p99_us | 4337 | 3332 … 4932 | 5 |
| crash | resident_delivery_samples | 1584 | 1584 … 1584 | 5 |
| crash | resident_during_handoff_p50_us | 1595 | 1072 … 2304 | 5 |
| crash | resident_during_handoff_p95_us | 2585 | 2525 … 2682 | 5 |
| crash | resident_during_handoff_p99_us | 2672 | 2578 … 2712 | 5 |
| crash | resident_during_handoff_samples | 144 | 144 … 144 | 5 |
| crash | source_exit_ms | 5.777 | 5.74 … 5.837 | 5 |
| crash | viewers | 144 | 144 … 144 | 5 |
| fixed | complete_viewers | 144 | 144 … 144 | 5 |
| fixed | connect_peak_qps | 480 | 460 … 480 | 5 |
| fixed | danmaku_delivery_queue_wait_seconds_count | 24 | 24 … 24 | 5 |
| fixed | danmaku_delivery_queue_wait_seconds_sum | 0.000192332 | 0.00016579300000000002 … 0.000197376 | 5 |
| fixed | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| fixed | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| fixed | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| fixed | danmaku_fanout_frames_total{mode="single"} | 3240 | 3240 … 3240 | 5 |
| fixed | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| fixed | drain_ms | 408.03 | 405.846 … 408.393 | 5 |
| fixed | events | 24 | 24 … 24 | 5 |
| fixed | go_gc_duration_seconds_sum | 0.001397166 | 0.0007354589999999999 … 0.001989751 | 5 |
| fixed | go_goroutines | 26 | 26 … 27 | 5 |
| fixed | go_memstats_alloc_bytes_total | 7183064 | 7102840 … 7928016 | 5 |
| fixed | invited | 72 | 72 … 72 | 5 |
| fixed | last_handoff_ms | 310 | 309 … 311 | 5 |
| fixed | migrating_delivery_p50_us | 2629 | 2603 … 2670 | 5 |
| fixed | migrating_delivery_p95_us | 3375 | 3166 … 4204 | 5 |
| fixed | migrating_delivery_p99_us | 4396 | 3628 … 4804 | 5 |
| fixed | migrating_delivery_samples | 1584 | 1584 … 1584 | 5 |
| fixed | moved | 72 | 72 … 72 | 5 |
| fixed | peak_attempts_per_sec | 72 | 72 … 72 | 5 |
| fixed | process_cpu_seconds_total | 0.18665900000000002 | 0.18166100000000002 … 0.19061399999999998 | 5 |
| fixed | process_resident_memory_bytes | 10993664 | 10452992 … 11288576 | 5 |
| fixed | recent_peak_qps | 480 | 390 … 480 | 5 |
| fixed | recovery_p50_ms | 7 | 7 … 8 | 5 |
| fixed | recovery_p95_ms | 10 | 9 … 10 | 5 |
| fixed | recovery_p99_ms | 10 | 10 … 10 | 5 |
| fixed | recovery_samples | 72 | 72 … 72 | 5 |
| fixed | resident_delivery_p50_us | 2624 | 2595 … 2676 | 5 |
| fixed | resident_delivery_p95_us | 3276 | 3184 … 4212 | 5 |
| fixed | resident_delivery_p99_ms | 4 | 3 … 4 | 5 |
| fixed | resident_delivery_p99_us | 4413 | 3672 … 4770 | 5 |
| fixed | resident_delivery_samples | 1584 | 1584 … 1584 | 5 |
| fixed | resident_during_handoff_p50_us | 2591 | 2424 … 2742 | 5 |
| fixed | resident_during_handoff_p95_us | 3244 | 2827 … 3376 | 5 |
| fixed | resident_during_handoff_p99_us | 3311 | 2913 … 3430 | 5 |
| fixed | resident_during_handoff_samples | 144 | 144 … 144 | 5 |
| fixed | source_exit_ms | 408.03 | 405.846 … 408.393 | 5 |
| fixed | viewers | 144 | 144 … 144 | 5 |
| bounded | complete_viewers | 144 | 144 … 144 | 5 |
| bounded | connect_peak_qps | 80 | 80 … 80 | 5 |
| bounded | danmaku_delivery_queue_wait_seconds_count | 24 | 24 … 24 | 5 |
| bounded | danmaku_delivery_queue_wait_seconds_sum | 0.00018896100000000005 | 0.00017745699999999998 … 0.00020370799999999998 | 5 |
| bounded | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| bounded | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| bounded | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| bounded | danmaku_fanout_frames_total{mode="single"} | 3027 | 3024 … 3028 | 5 |
| bounded | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| bounded | drain_ms | 1907.667 | 1907.365 … 1907.816 | 5 |
| bounded | events | 24 | 24 … 24 | 5 |
| bounded | go_gc_duration_seconds_sum | 0.001035541 | 0.0007436259999999999 … 0.001411541 | 5 |
| bounded | go_goroutines | 6 | 5 … 8 | 5 |
| bounded | go_memstats_alloc_bytes_total | 7604680 | 7125280 … 8453816 | 5 |
| bounded | invited | 72 | 72 … 72 | 5 |
| bounded | last_handoff_ms | 1805 | 1804 … 1806 | 5 |
| bounded | migrating_delivery_p50_us | 2635 | 2582 … 2652 | 5 |
| bounded | migrating_delivery_p95_us | 3470 | 3300 … 4172 | 5 |
| bounded | migrating_delivery_p99_us | 4635 | 4042 … 4860 | 5 |
| bounded | migrating_delivery_samples | 1584 | 1584 … 1584 | 5 |
| bounded | moved | 72 | 72 … 72 | 5 |
| bounded | peak_attempts_per_sec | 36 | 36 … 36 | 5 |
| bounded | process_cpu_seconds_total | 0.21680699999999997 | 0.20936500000000002 … 0.22312 | 5 |
| bounded | process_resident_memory_bytes | 10403840 | 9879552 … 10993664 | 5 |
| bounded | recent_peak_qps | 80 | 80 … 80 | 5 |
| bounded | recovery_p50_ms | 3 | 3 … 3 | 5 |
| bounded | recovery_p95_ms | 5 | 5 … 6 | 5 |
| bounded | recovery_p99_ms | 5 | 5 … 6 | 5 |
| bounded | recovery_samples | 72 | 72 … 72 | 5 |
| bounded | resident_delivery_p50_us | 2644 | 2631 … 2700 | 5 |
| bounded | resident_delivery_p95_us | 3416 | 3240 … 3773 | 5 |
| bounded | resident_delivery_p99_ms | 4 | 3 … 4 | 5 |
| bounded | resident_delivery_p99_us | 4431 | 3966 … 4672 | 5 |
| bounded | resident_delivery_samples | 1584 | 1584 … 1584 | 5 |
| bounded | resident_during_handoff_p50_us | 2533 | 2427 … 2676 | 5 |
| bounded | resident_during_handoff_p95_us | 3852 | 3046 … 4096 | 5 |
| bounded | resident_during_handoff_p99_us | 4060 | 3137 … 4919 | 5 |
| bounded | resident_during_handoff_samples | 576 | 576 … 576 | 5 |
| bounded | source_exit_ms | 1907.667 | 1907.365 … 1907.816 | 5 |
| bounded | viewers | 144 | 144 … 144 | 5 |

## history_pagination page_1 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| offset_baseline | p50_us | 344.75 | 325.709 … 360.375 | 5 |
| offset_baseline | p95_us | 388.708 | 350.209 … 391.25 | 5 |
| offset_baseline | p999_us | 388.708 | 350.209 … 391.25 | 5 |
| offset_baseline | p99_us | 388.708 | 350.209 … 391.25 | 5 |
| offset_baseline | samples | 6 | 6 … 6 | 5 |
| cursor_before_id_prod | p50_us | 355.583 | 330.875 … 371.333 | 5 |
| cursor_before_id_prod | p95_us | 378.917 | 368.292 … 390.084 | 5 |
| cursor_before_id_prod | p999_us | 378.917 | 368.292 … 390.084 | 5 |
| cursor_before_id_prod | p99_us | 378.917 | 368.292 … 390.084 | 5 |
| cursor_before_id_prod | samples | 6 | 6 … 6 | 5 |

## history_pagination page_200 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| offset_baseline | p50_us | 8623.5 | 8576 … 8816.708 | 5 |
| offset_baseline | p95_us | 8799.834 | 8728.542 … 9067.625 | 5 |
| offset_baseline | p999_us | 8799.834 | 8728.542 … 9067.625 | 5 |
| offset_baseline | p99_us | 8799.834 | 8728.542 … 9067.625 | 5 |
| offset_baseline | samples | 6 | 6 … 6 | 5 |
| cursor_before_id_prod | p50_us | 354.375 | 339.166 … 363.334 | 5 |
| cursor_before_id_prod | p95_us | 429.875 | 377.917 … 444.667 | 5 |
| cursor_before_id_prod | p999_us | 429.875 | 377.917 … 444.667 | 5 |
| cursor_before_id_prod | p99_us | 429.875 | 377.917 … 444.667 | 5 |
| cursor_before_id_prod | samples | 6 | 6 … 6 | 5 |

## history_pagination page_1000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| offset_baseline | p50_us | 36112.333 | 35293.583 … 36357.875 | 5 |
| offset_baseline | p95_us | 36882.667 | 36537.334 … 43764.708 | 5 |
| offset_baseline | p999_us | 36882.667 | 36537.334 … 43764.708 | 5 |
| offset_baseline | p99_us | 36882.667 | 36537.334 … 43764.708 | 5 |
| offset_baseline | samples | 6 | 6 … 6 | 5 |
| cursor_before_id_prod | p50_us | 386.916 | 376.25 … 402.25 | 5 |
| cursor_before_id_prod | p95_us | 442.542 | 418.375 … 447.417 | 5 |
| cursor_before_id_prod | p999_us | 442.542 | 418.375 … 447.417 | 5 |
| cursor_before_id_prod | p99_us | 442.542 | 418.375 … 447.417 | 5 |
| cursor_before_id_prod | samples | 6 | 6 … 6 | 5 |

## history_pagination page_2000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| offset_baseline | p50_us | 86538.125 | 85840.375 … 86898.25 | 5 |
| offset_baseline | p95_us | 87514.25 | 86784.625 … 87908.875 | 5 |
| offset_baseline | p999_us | 87514.25 | 86784.625 … 87908.875 | 5 |
| offset_baseline | p99_us | 87514.25 | 86784.625 … 87908.875 | 5 |
| offset_baseline | samples | 6 | 6 … 6 | 5 |
| cursor_before_id_prod | p50_us | 398.333 | 371.042 … 402.333 | 5 |
| cursor_before_id_prod | p95_us | 462.458 | 445.208 … 532.083 | 5 |
| cursor_before_id_prod | p999_us | 462.458 | 445.208 … 532.083 | 5 |
| cursor_before_id_prod | p99_us | 462.458 | 445.208 … 532.083 | 5 |
| cursor_before_id_prod | samples | 6 | 6 … 6 | 5 |

## history_pagination page_3600 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| offset_baseline | p50_us | 155210.083 | 154404.625 … 155857.875 | 5 |
| offset_baseline | p95_us | 156564.459 | 155893.708 … 157101.417 | 5 |
| offset_baseline | p999_us | 156564.459 | 155893.708 … 157101.417 | 5 |
| offset_baseline | p99_us | 156564.459 | 155893.708 … 157101.417 | 5 |
| offset_baseline | samples | 6 | 6 … 6 | 5 |
| cursor_before_id_prod | p50_us | 383.959 | 357.875 … 404.166 | 5 |
| cursor_before_id_prod | p95_us | 473.334 | 418 … 545.875 | 5 |
| cursor_before_id_prod | p999_us | 473.334 | 418 … 545.875 | 5 |
| cursor_before_id_prod | p99_us | 473.334 | 418 … 545.875 | 5 |
| cursor_before_id_prod | samples | 6 | 6 … 6 | 5 |

## pg_archive_write batch_128 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| row_by_row_tx | alloc_bytes_per_batch | 72319.04 | 72296 … 72319.04 | 5 |
| row_by_row_tx | allocs_per_batch | 1676.01 | 1676 … 1676.01 | 5 |
| row_by_row_tx | batch_p50_us | 20569.333 | 20355.667 … 20684.709 | 5 |
| row_by_row_tx | batch_p95_us | 21326.208 | 20927.416 … 23463.625 | 5 |
| row_by_row_tx | batch_p999_us | 28525.625 | 27109.125 … 40132.167 | 5 |
| row_by_row_tx | batch_p99_us | 25746.958 | 22489.458 … 28259.083 | 5 |
| row_by_row_tx | batch_samples | 100 | 100 … 100 | 5 |
| row_by_row_tx | duplicate_rows | 0 | 0 … 0 | 5 |
| row_by_row_tx | rows_per_second | 6291.176410054454 | 6194.386501577762 … 6352.034201734155 | 5 |
| multivalue_insert | alloc_bytes_per_batch | 159367.04 | 159326.64 … 159367.2 | 5 |
| multivalue_insert | allocs_per_batch | 1328.45 | 1328.37 … 1328.45 | 5 |
| multivalue_insert | batch_p50_us | 1292.542 | 1273.334 … 1310.291 | 5 |
| multivalue_insert | batch_p95_us | 1521.25 | 1475.25 … 1652.708 | 5 |
| multivalue_insert | batch_p999_us | 7689.583 | 1739.416 … 25846.833 | 5 |
| multivalue_insert | batch_p99_us | 1880.834 | 1559.25 … 2349.75 | 5 |
| multivalue_insert | batch_samples | 100 | 100 … 100 | 5 |
| multivalue_insert | duplicate_rows | 0 | 0 … 0 | 5 |
| multivalue_insert | rows_per_second | 93155.55362516547 | 80965.3432086425 … 99331.74182671461 | 5 |
| unnest_prod | alloc_bytes_per_batch | 73960.64 | 73937.76 … 73961.28 | 5 |
| unnest_prod | allocs_per_batch | 809.94 | 809.93 … 809.95 | 5 |
| unnest_prod | batch_p50_us | 1232.583 | 1214.792 … 1247.625 | 5 |
| unnest_prod | batch_p95_us | 1350.792 | 1325.417 … 1418.708 | 5 |
| unnest_prod | batch_p999_us | 1534.958 | 1488.417 … 30919.458 | 5 |
| unnest_prod | batch_p99_us | 1495.541 | 1468.916 … 1836.166 | 5 |
| unnest_prod | batch_samples | 100 | 100 … 100 | 5 |
| unnest_prod | duplicate_rows | 0 | 0 … 0 | 5 |
| unnest_prod | rows_per_second | 103448.51943428337 | 81890.17048996092 … 104762.64230961422 | 5 |

## redis_lua_atomicity 0s · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| multi_cmd_baseline | extra_broadcasts | 35 | 35 … 35 | 5 |
| multi_cmd_baseline | p50_us | 718.25 | 714.791 … 720.417 | 5 |
| multi_cmd_baseline | p95_us | 738.208 | 733.625 … 1122 | 5 |
| multi_cmd_baseline | p999_us | 764.875 | 751.334 … 1620.125 | 5 |
| multi_cmd_baseline | p99_us | 752.292 | 737.833 … 1415.375 | 5 |
| multi_cmd_baseline | qps | 1392.0621138115184 | 1291.2236548248243 … 1396.163794250469 | 5 |
| multi_cmd_baseline | race_duplicate_rate | 0.21875 | 0.21875 … 0.21875 | 5 |
| multi_cmd_baseline | samples | 100 | 100 … 100 | 5 |
| multi_cmd_baseline | stream_writes | 40 | 40 … 40 | 5 |
| lua_evalsha_prod | extra_broadcasts | 0 | 0 … 0 | 5 |
| lua_evalsha_prod | p50_us | 93.583 | 93.333 … 94.917 | 5 |
| lua_evalsha_prod | p95_us | 106.042 | 105.167 … 107.417 | 5 |
| lua_evalsha_prod | p999_us | 118.583 | 109.333 … 411.208 | 5 |
| lua_evalsha_prod | p99_us | 115.625 | 109.167 … 136 | 5 |
| lua_evalsha_prod | qps | 10544.02753593879 | 10173.50091937928 … 10559.801475732254 | 5 |
| lua_evalsha_prod | race_duplicate_rate | 0 | 0 … 0 | 5 |
| lua_evalsha_prod | samples | 100 | 100 … 100 | 5 |
| lua_evalsha_prod | stream_writes | 5 | 5 … 5 | 5 |

## redis_lua_atomicity 2ms · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| multi_cmd_baseline | extra_broadcasts | 35 | 35 … 35 | 5 |
| multi_cmd_baseline | p50_us | 26711.375 | 26257.958 … 26798.292 | 5 |
| multi_cmd_baseline | p95_us | 27270.5 | 27227.209 … 27293.25 | 5 |
| multi_cmd_baseline | p999_us | 27453.458 | 27382.542 … 27514.667 | 5 |
| multi_cmd_baseline | p99_us | 27389.875 | 27374.125 … 27489.083 | 5 |
| multi_cmd_baseline | qps | 37.535917180752335 | 37.40820377029078 … 38.44261963599451 | 5 |
| multi_cmd_baseline | race_duplicate_rate | 0.21875 | 0.21875 … 0.21875 | 5 |
| multi_cmd_baseline | samples | 100 | 100 … 100 | 5 |
| multi_cmd_baseline | stream_writes | 40 | 40 … 40 | 5 |
| lua_evalsha_prod | extra_broadcasts | 0 | 0 … 0 | 5 |
| lua_evalsha_prod | p50_us | 3030.5 | 3015.792 … 3052.75 | 5 |
| lua_evalsha_prod | p95_us | 3208.791 | 3190.042 … 3250.041 | 5 |
| lua_evalsha_prod | p999_us | 3358.292 | 3300.5 … 3609.291 | 5 |
| lua_evalsha_prod | p99_us | 3352.125 | 3263.75 … 3588.625 | 5 |
| lua_evalsha_prod | qps | 331.6536458063433 | 329.0129361536546 … 332.150829877852 | 5 |
| lua_evalsha_prod | race_duplicate_rate | 0 | 0 … 0 | 5 |
| lua_evalsha_prod | samples | 100 | 100 … 100 | 5 |
| lua_evalsha_prod | stream_writes | 5 | 5 … 5 | 5 |

## sensitive_filter_scaling words_100 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| naive_contains | hit_us | 0.85083 | 0.82041 … 1.00583 | 5 |
| naive_contains | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| naive_contains | miss_us | 0.635 | 0.63333 … 0.68 | 5 |
| merged_regexp | hit_us | 95.47792 | 94.26959000000001 … 98.19709 | 5 |
| merged_regexp | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| merged_regexp | miss_us | 95.99166 | 95.9425 … 96.36958 | 5 |
| ac_prod | hit_us | 1.34333 | 1.3133299999999999 … 1.42791 | 5 |
| ac_prod | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| ac_prod | miss_us | 1.32875 | 1.2625 … 1.42625 | 5 |

## sensitive_filter_scaling words_1000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| naive_contains | hit_us | 9.1825 | 8.63167 … 9.508329999999999 | 5 |
| naive_contains | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| naive_contains | miss_us | 6.525410000000001 | 6.34709 … 6.865 | 5 |
| merged_regexp | hit_us | 1520.645 | 1439.7095800000002 … 1524.21666 | 5 |
| merged_regexp | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| merged_regexp | miss_us | 1486.92 | 1442.6954199999998 … 1517.13958 | 5 |
| ac_prod | hit_us | 1.51625 | 1.3825 … 2.00833 | 5 |
| ac_prod | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| ac_prod | miss_us | 1.8525 | 1.39833 … 2.5629199999999996 | 5 |

## sensitive_filter_scaling words_5000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| naive_contains | hit_us | 46.14584 | 43.8925 … 46.50375 | 5 |
| naive_contains | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| naive_contains | miss_us | 33.825 | 33.729169999999996 … 34.244589999999995 | 5 |
| ac_prod | hit_us | 1.43333 | 1.32625 … 1.6029200000000001 | 5 |
| ac_prod | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| ac_prod | miss_us | 1.39458 | 1.32917 … 1.55959 | 5 |

## sensitive_filter_scaling words_20000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| naive_contains | hit_us | 184.48583 | 183.08125 … 185.27834 | 5 |
| naive_contains | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| naive_contains | miss_us | 137.0575 | 135.6525 … 137.23584 | 5 |
| ac_prod | hit_us | 1.53875 | 1.45959 … 1.60459 | 5 |
| ac_prod | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| ac_prod | miss_us | 1.47125 | 1.41167 … 1.5425 | 5 |

## sensitive_filter_scaling reload_5000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| rwmutex_rebuild_baseline | reload_p50_us | 2 | 1.958 … 2.083 | 5 |
| rwmutex_rebuild_baseline | reload_p95_us | 13474.833 | 11312.125 … 13883.875 | 5 |
| rwmutex_rebuild_baseline | reload_p999_us | 27927.375 | 20453.166 … 46281.375 | 5 |
| rwmutex_rebuild_baseline | reload_p99_us | 25336.75 | 18316.666 … 30292.875 | 5 |
| rwmutex_rebuild_baseline | reload_samples | 160 | 160 … 160 | 5 |
| atomic_pointer_prod | reload_p50_us | 1.542 | 1.542 … 1.584 | 5 |
| atomic_pointer_prod | reload_p95_us | 4.292 | 4.042 … 4.375 | 5 |
| atomic_pointer_prod | reload_p999_us | 93.042 | 80.875 … 95.708 | 5 |
| atomic_pointer_prod | reload_p99_us | 9.542 | 7.959 … 10.083 | 5 |
| atomic_pointer_prod | reload_samples | 267486 | 244327 … 299335 | 5 |

## live_source_singleflight burst_120 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| direct_no_coalesce | cold_p50_us | 85352.917 | 84203.416 … 88315.5 | 5 |
| direct_no_coalesce | cold_p95_us | 87578.041 | 85887.291 … 89519.958 | 5 |
| direct_no_coalesce | cold_p999_us | 87659.083 | 86858.084 … 90414.166 | 5 |
| direct_no_coalesce | cold_p99_us | 87619.792 | 86687.25 … 89855.459 | 5 |
| direct_no_coalesce | cold_samples | 120 | 120 … 120 | 5 |
| direct_no_coalesce | cold_upstream_calls | 120 | 120 … 120 | 5 |
| direct_no_coalesce | warm_p50_us | 84837.958 | 84303.792 … 86498.625 | 5 |
| direct_no_coalesce | warm_p95_us | 87339 | 85370.167 … 88255.917 | 5 |
| direct_no_coalesce | warm_p999_us | 88050.917 | 86635.708 … 89403.5 | 5 |
| direct_no_coalesce | warm_p99_us | 87933.959 | 86134.458 … 88910.75 | 5 |
| direct_no_coalesce | warm_samples | 120 | 120 … 120 | 5 |
| direct_no_coalesce | warm_upstream_calls | 120 | 120 … 120 | 5 |
| prod_singleflight_cache | cold_p50_us | 82758.459 | 81640 … 82926.375 | 5 |
| prod_singleflight_cache | cold_p95_us | 82854.833 | 81726.5 … 83079.791 | 5 |
| prod_singleflight_cache | cold_p999_us | 82890.125 | 81773 … 83111 | 5 |
| prod_singleflight_cache | cold_p99_us | 82889.25 | 81755.084 … 83092.5 | 5 |
| prod_singleflight_cache | cold_samples | 120 | 120 … 120 | 5 |
| prod_singleflight_cache | cold_upstream_calls | 1 | 1 … 1 | 5 |
| prod_singleflight_cache | warm_p50_us | 0.666 | 0.5 … 0.791 | 5 |
| prod_singleflight_cache | warm_p95_us | 9.208 | 3.708 … 19.167 | 5 |
| prod_singleflight_cache | warm_p999_us | 34 | 22.334 … 47.875 | 5 |
| prod_singleflight_cache | warm_p99_us | 31.917 | 9.625 … 36.958 | 5 |
| prod_singleflight_cache | warm_samples | 120 | 120 … 120 | 5 |
| prod_singleflight_cache | warm_upstream_calls | 0 | 0 … 0 | 5 |

