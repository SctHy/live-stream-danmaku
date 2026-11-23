# Maku Evaluation

darwin/arm64 · go1.27.4 · CPUs 10 · profile full · repeats 5 (+ warmup) · 2026-11-23T11:24:52.059372Z · git_sha c81541ed51f170e9e45d59c9f016beffc985fcdc · dirty true

## Scorecard

| Experiment / load | Metric | Baseline → treatment | Medians | Change (%) | Δ 95% CI | Trade-off | Gate |
| --- | --- | --- | --- | --- | --- | --- | --- |
| fanout_sweep low | frames | legacy → adaptive_batch | 6400 → 6400 | 0 | 0 … 0 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | hot_p50_us | legacy → adaptive_batch | 2545 → 2568 | 0.9037328094302577 | -404 … 377 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | hot_p99_us | legacy → adaptive_batch | 4361 → 4554 | 4.425590460903472 | -1618 … 672 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | process_cpu_seconds_total | legacy → adaptive_batch | 0.42537899999999995 → 0.45509499999999997 | 6.985770336570463 | -0.00323699999999999 … 0.06653399999999998 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | frames | legacy → mixed_25pct_legacy | 6400 → 6400 | 0 | 0 … 0 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | hot_p50_us | legacy → mixed_25pct_legacy | 2545 → 2436 | -4.282907662082513 | -325 … 443 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | hot_p99_us | legacy → mixed_25pct_legacy | 4361 → 4684 | 7.406558128869523 | -1488 … 453 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep low | process_cpu_seconds_total | legacy → mixed_25pct_legacy | 0.42537899999999995 → 0.443903 | 4.354704863192604 | -0.01048899999999997 … 0.05908899999999995 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | frames | legacy → adaptive_batch | 76800 → 76800 | 0 | 0 … 0 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | hot_p50_us | legacy → adaptive_batch | 1839 → 1898 | 3.2082653616095635 | -140 … 319 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | hot_p99_us | legacy → adaptive_batch | 5650 → 5775 | 2.212389380530966 | -897 … 693 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | process_cpu_seconds_total | legacy → adaptive_batch | 1.479955 → 1.50575 | 1.7429584007621957 | -0.013552999999999926 … 0.10365499999999983 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | frames | legacy → mixed_25pct_legacy | 76800 → 76800 | 0 | 0 … 0 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | hot_p50_us | legacy → mixed_25pct_legacy | 1839 → 1801 | -2.066340402392608 | -486 … 177 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | hot_p99_us | legacy → mixed_25pct_legacy | 5650 → 4808 | -14.902654867256636 | -1605 … 3197 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep medium | process_cpu_seconds_total | legacy → mixed_25pct_legacy | 1.479955 → 1.470658 | -0.6281947761925144 | -0.15128199999999992 … 0.06856299999999993 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | frames | legacy → adaptive_batch | 313600 → 103712 | -66.92857142857143 | -213920 … -209664 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | hot_p50_us | legacy → adaptive_batch | 1571 → 4807 | 205.98345003182686 | 2776 … 3440 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | hot_p99_us | legacy → adaptive_batch | 9273 → 9837 | 6.082174053704303 | -7963 … 5650 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | process_cpu_seconds_total | legacy → adaptive_batch | 3.159016 → 1.547283 | -51.020096131200354 | -1.7611529999999997 … -1.5077820000000002 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | frames | legacy → mixed_25pct_legacy | 313600 → 150808 | -51.910714285714285 | -164136 … -161616 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | hot_p50_us | legacy → mixed_25pct_legacy | 1571 → 3411 | 117.12285168682368 | 1692 … 2044 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | hot_p99_us | legacy → mixed_25pct_legacy | 9273 → 11451 | 23.487544483985758 | -6349 … 18196 | Batching can increase hot-room latency while reducing frames. | PASS |
| fanout_sweep high | process_cpu_seconds_total | legacy → mixed_25pct_legacy | 3.159016 → 1.97927 | -37.345363239692354 | -1.323299 … -1.0833109999999997 | Batching can increase hot-room latency while reducing frames. | PASS |
| isolation_stalled | cold_p99_us | baseline_single_shard_unbounded → protected_default | 8902 → 6086 | -31.633340822287124 | -16945 … 3975 | Bounded queues can disconnect stalled clients to limit resident memory. | PASS |
| isolation_stalled | rss_peak_bytes | baseline_single_shard_unbounded → protected_default | 70189056 → 48103424 | -31.465919701213817 | -24100864 … -19759104 | Bounded queues can disconnect stalled clients to limit resident memory. | PASS |
| migration_strategies | connect_peak_qps | crash → fixed | 310 → 450 | 45.16129032258065 | -80 … 190 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| migration_strategies | drain_ms | crash → fixed | 454 → 407.609 | -10.21828193832599 | -49.47000000000003 … -37.39100000000002 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| migration_strategies | recent_peak_qps | crash → fixed | 300 → 480 | 60.00000000000001 | -80 … 210 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| migration_strategies | connect_peak_qps | crash → bounded | 310 → 80 | -74.19354838709677 | -270 … -190 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| migration_strategies | drain_ms | crash → bounded | 454 → 1905.344 | 319.679295154185 | 1447.342 … 1460.344 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| migration_strategies | recent_peak_qps | crash → bounded | 300 → 80 | -73.33333333333334 | -270 … -190 | Pacing handoffs spreads recovery requests and can extend drain time. | PASS |
| history_pagination page_1 | p50_us | offset_baseline → cursor_before_id_prod | 518.583 → 514.333 | -0.8195409413729315 | -43.99899999999997 … 26.541999999999973 | Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps. | PASS |
| history_pagination page_200 | p50_us | offset_baseline → cursor_before_id_prod | 9178.834 → 533.834 | -94.18407610378398 | -8821.625 … -8542.249 | Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps. | PASS |
| history_pagination page_1000 | p50_us | offset_baseline → cursor_before_id_prod | 37609.5 → 523.292 | -98.60861750355629 | -40292.416 … -36738.208 | Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps. | PASS |
| history_pagination page_2000 | p50_us | offset_baseline → cursor_before_id_prod | 90843.583 → 581.542 | -99.35984251083535 | -90488.583 … -89323.541 | Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps. | PASS |
| history_pagination page_3600 | p50_us | offset_baseline → cursor_before_id_prod | 162105.792 → 552.792 | -99.65899306053173 | -161930.874 … -160003.95799999998 | Cursor requires the previous page boundary. OFFSET supports arbitrary page jumps. | PASS |
| pg_archive_write batch_128 | rows_per_second | row_by_row_tx → multivalue_insert | 4489.995356916848 → 80253.73975373701 | 1687.3902615535212 | 72049.12106475988 … 78096.71060295602 | UNNEST builds client arrays but uses five bind parameters per batch. | PASS |
| pg_archive_write batch_128 | rows_per_second | row_by_row_tx → unnest_prod | 4489.995356916848 → 80562.93349781599 | 1694.2765435983936 | 68263.11299121815 … 77148.9925357215 | UNNEST builds client arrays but uses five bind parameters per batch. | PASS |
| redis_lua_atomicity 0s | p50_us | multi_cmd_baseline → lua_evalsha_prod | 3414.625 → 386.417 | -88.68347183072812 | -3203.042 … -2296.583 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| redis_lua_atomicity 0s | qps | multi_cmd_baseline → lua_evalsha_prod | 295.03496717551593 → 2586.0954492142187 | 776.5386265810838 | 1994.1551537760597 … 2489.7683796059755 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| redis_lua_atomicity 0s | race_duplicate_rate | multi_cmd_baseline → lua_evalsha_prod | 0.21875 → 0 | -100 | -0.21875 … -0.21875 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| redis_lua_atomicity 2ms | p50_us | multi_cmd_baseline → lua_evalsha_prod | 34824 → 3643.625 | -89.5370290604181 | -31794.708 … -29355.083 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| redis_lua_atomicity 2ms | qps | multi_cmd_baseline → lua_evalsha_prod | 29.47157611675259 → 264.74762900495386 | 798.315135763855 | 202.58418105245454 … 243.5373298678448 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| redis_lua_atomicity 2ms | race_duplicate_rate | multi_cmd_baseline → lua_evalsha_prod | 0.21875 → 0 | -100 | -0.21875 … -0.21875 | Lua serializes policy and publication on the Redis server within one round trip. | PASS |
| sensitive_filter_scaling words_100 | miss_us | naive_contains → merged_regexp | 0.70041 → 96.38958000000001 | 13661.879470595795 | 95.11417 … 107.72124999999998 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling words_100 | miss_us | naive_contains → ac_prod | 0.70041 → 1.35125 | 92.92271669450751 | 0.5320900000000001 … 0.75292 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling words_1000 | miss_us | naive_contains → merged_regexp | 6.5804100000000005 → 1475.76959 | 22326.711861418968 | 1448.2450099999999 … 1621.16959 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling words_1000 | miss_us | naive_contains → ac_prod | 6.5804100000000005 → 1.36625 | -79.23761589323462 | -5.56583 … -4.98167 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling words_5000 | miss_us | naive_contains → ac_prod | 34.42792 → 1.41792 | -95.88148223883405 | -33.39834 … -32.359159999999996 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling words_20000 | miss_us | naive_contains → ac_prod | 137.23083 → 1.31166 | -99.04419436944308 | -137.46875 … -132.63959 | AC builds a trie before matching. Regexp is limited to 1,000 alternatives to bound compilation and scan costs. | PASS |
| sensitive_filter_scaling reload_5000 | reload_p99_us | rwmutex_rebuild_baseline → atomic_pointer_prod | 23408.709 → 9.083 | -99.96119820191707 | -30656.958000000002 … -18749.417 | Atomic reload temporarily retains old and new tries. | PASS |
| live_source_singleflight burst_120 | cold_upstream_calls | direct_no_coalesce → prod_singleflight_cache | 120 → 1 | -99.16666666666667 | -119 … -119 | Live results may remain cached for 15 seconds. | PASS |
| live_source_singleflight burst_120 | warm_upstream_calls | direct_no_coalesce → prod_singleflight_cache | 120 → 0 | -100 | -120 … -120 | Live results may remain cached for 15 seconds. | PASS |

## fanout_sweep low · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| legacy | achieved_rate | 99.96940311455926 | 99.92555546118142 … 99.9847835657511 | 5 |
| legacy | affected | 0 | 0 … 0 | 5 |
| legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| legacy | cold_gaps | 0 | 0 … 0 | 5 |
| legacy | cold_missing | 0 | 0 … 0 | 5 |
| legacy | cold_p99_us | 4356 | 4319 … 6430 | 5 |
| legacy | cold_rate | 100 | 100 … 100 | 5 |
| legacy | cold_received | 200 | 200 … 200 | 5 |
| legacy | cold_samples | 200 | 200 … 200 | 5 |
| legacy | cold_unique | 200 | 200 … 200 | 5 |
| legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_count | 400 | 400 … 400 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_sum | 0.0020640029999999987 | 0.0018074569999999993 … 0.0027543360000000005 | 5 |
| legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="single"} | 10400 | 10400 … 10400 | 5 |
| legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| legacy | duplicates | 0 | 0 … 0 | 5 |
| legacy | expected | 6400 | 6400 … 6400 | 5 |
| legacy | frames | 6400 | 6400 … 6400 | 5 |
| legacy | go_gc_duration_seconds_sum | 0.000534917 | 0.00045587499999999995 … 0.00056054 | 5 |
| legacy | go_goroutines | -1 | -1 … -1 | 5 |
| legacy | go_memstats_alloc_bytes_total | 13950304 | 12850456 … 15077984 | 5 |
| legacy | hot_p50_us | 2545 | 2252 … 2557 | 5 |
| legacy | hot_p99_us | 4361 | 4238 … 6172 | 5 |
| legacy | hot_samples | 1600 | 1600 … 1600 | 5 |
| legacy | missing | 0 | 0 … 0 | 5 |
| legacy | payload_bytes | 2194528 | 2193312 … 2195040 | 5 |
| legacy | process_cpu_seconds_total | 0.42537899999999995 | 0.410401 … 0.45439199999999996 | 5 |
| legacy | process_resident_memory_bytes | 11042816 | 10928128 … 12156928 | 5 |
| legacy | received | 6400 | 6400 … 6400 | 5 |
| legacy | rss_peak_bytes | 35504128 | 35127296 … 36454400 | 5 |
| legacy | seconds | 2 | 2 … 2 | 5 |
| legacy | target_rate | 100 | 100 … 100 | 5 |
| legacy | viewers | 32 | 32 … 32 | 5 |
| legacy | worst_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | achieved_rate | 99.92879863246961 | 99.89606643424868 … 99.96436060619234 | 5 |
| adaptive_batch | affected | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_p99_us | 4294 | 4151 … 4975 | 5 |
| adaptive_batch | cold_rate | 100 | 100 … 100 | 5 |
| adaptive_batch | cold_received | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_samples | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_unique | 200 | 200 … 200 | 5 |
| adaptive_batch | cursor_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_count | 400 | 400 … 400 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_sum | 0.002245031999999999 | 0.0020938669999999997 … 0.006586754999999999 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="single"} | 10400 | 10400 … 10400 | 5 |
| adaptive_batch | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| adaptive_batch | duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | expected | 6400 | 6400 … 6400 | 5 |
| adaptive_batch | frames | 6400 | 6400 … 6400 | 5 |
| adaptive_batch | go_gc_duration_seconds_sum | 0.000485752 | 0.00046520599999999996 … 0.000680041 | 5 |
| adaptive_batch | go_goroutines | -1 | -2 … 0 | 5 |
| adaptive_batch | go_memstats_alloc_bytes_total | 14944320 | 13915848 … 17131776 | 5 |
| adaptive_batch | hot_p50_us | 2568 | 2141 … 2655 | 5 |
| adaptive_batch | hot_p99_us | 4554 | 4070 … 5033 | 5 |
| adaptive_batch | hot_samples | 1600 | 1600 … 1600 | 5 |
| adaptive_batch | missing | 0 | 0 … 0 | 5 |
| adaptive_batch | payload_bytes | 2194528 | 2193472 … 2195072 | 5 |
| adaptive_batch | process_cpu_seconds_total | 0.45509499999999997 | 0.427374 … 0.47990000000000005 | 5 |
| adaptive_batch | process_resident_memory_bytes | 11354112 | 10862592 … 12877824 | 5 |
| adaptive_batch | received | 6400 | 6400 … 6400 | 5 |
| adaptive_batch | rss_peak_bytes | 35504128 | 35258368 … 36323328 | 5 |
| adaptive_batch | seconds | 2 | 2 … 2 | 5 |
| adaptive_batch | target_rate | 100 | 100 … 100 | 5 |
| adaptive_batch | viewers | 32 | 32 … 32 | 5 |
| adaptive_batch | worst_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | achieved_rate | 99.93463235723661 | 99.90544783572838 … 99.99074255708186 | 5 |
| mixed_25pct_legacy | affected | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_p99_us | 4463 | 4178 … 5071 | 5 |
| mixed_25pct_legacy | cold_rate | 100 | 100 … 100 | 5 |
| mixed_25pct_legacy | cold_received | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_samples | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_unique | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_count | 600 | 600 … 600 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_sum | 0.005898161999999998 | 0.004771898000000002 … 0.010692086 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="single"} | 10400 | 10400 … 10400 | 5 |
| mixed_25pct_legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | expected | 6400 | 6400 … 6400 | 5 |
| mixed_25pct_legacy | frames | 6400 | 6400 … 6400 | 5 |
| mixed_25pct_legacy | go_gc_duration_seconds_sum | 0.00043187399999999995 | 0.000352125 … 0.000519334 | 5 |
| mixed_25pct_legacy | go_goroutines | -1 | -2 … 0 | 5 |
| mixed_25pct_legacy | go_memstats_alloc_bytes_total | 13304040 | 12465192 … 15162280 | 5 |
| mixed_25pct_legacy | hot_p50_us | 2436 | 2220 … 2773 | 5 |
| mixed_25pct_legacy | hot_p99_us | 4684 | 4445 … 4799 | 5 |
| mixed_25pct_legacy | hot_samples | 1600 | 1600 … 1600 | 5 |
| mixed_25pct_legacy | missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | payload_bytes | 2194784 | 2194208 … 2195072 | 5 |
| mixed_25pct_legacy | process_cpu_seconds_total | 0.443903 | 0.432319 … 0.48271800000000004 | 5 |
| mixed_25pct_legacy | process_resident_memory_bytes | 11468800 | 11337728 … 12435456 | 5 |
| mixed_25pct_legacy | received | 6400 | 6400 … 6400 | 5 |
| mixed_25pct_legacy | rss_peak_bytes | 35930112 | 35766272 … 36765696 | 5 |
| mixed_25pct_legacy | seconds | 2 | 2 … 2 | 5 |
| mixed_25pct_legacy | target_rate | 100 | 100 … 100 | 5 |
| mixed_25pct_legacy | viewers | 32 | 32 … 32 | 5 |
| mixed_25pct_legacy | worst_missing | 0 | 0 … 0 | 5 |

## fanout_sweep medium · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| legacy | achieved_rate | 399.9496731327807 | 399.9205407875523 … 399.99181676741665 | 5 |
| legacy | affected | 0 | 0 … 0 | 5 |
| legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| legacy | cold_gaps | 0 | 0 … 0 | 5 |
| legacy | cold_missing | 0 | 0 … 0 | 5 |
| legacy | cold_p99_us | 5345 | 4736 … 6238 | 5 |
| legacy | cold_rate | 100 | 100 … 100 | 5 |
| legacy | cold_received | 200 | 200 … 200 | 5 |
| legacy | cold_samples | 200 | 200 … 200 | 5 |
| legacy | cold_unique | 200 | 200 … 200 | 5 |
| legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_count | 1000 | 1000 … 1000 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_sum | 0.005692271000000006 | 0.003711663 … 0.008407141000000003 | 5 |
| legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="single"} | 80800 | 80800 … 80800 | 5 |
| legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| legacy | duplicates | 0 | 0 … 0 | 5 |
| legacy | expected | 76800 | 76800 … 76800 | 5 |
| legacy | frames | 76800 | 76800 … 76800 | 5 |
| legacy | go_gc_duration_seconds_sum | 0.000498333 | 0.00048633399999999994 … 0.0005543340000000001 | 5 |
| legacy | go_goroutines | 0 | -1 … 0 | 5 |
| legacy | go_memstats_alloc_bytes_total | 20587296 | 19673392 … 21253808 | 5 |
| legacy | hot_p50_us | 1839 | 1658 … 2038 | 5 |
| legacy | hot_p99_us | 5650 | 5199 … 5997 | 5 |
| legacy | hot_samples | 6400 | 6400 … 6400 | 5 |
| legacy | missing | 0 | 0 … 0 | 5 |
| legacy | payload_bytes | 26373696 | 26367744 … 26378784 | 5 |
| legacy | process_cpu_seconds_total | 1.479955 | 1.402095 … 1.4969180000000002 | 5 |
| legacy | process_resident_memory_bytes | 17235968 | 15613952 … 18038784 | 5 |
| legacy | received | 76800 | 76800 … 76800 | 5 |
| legacy | rss_peak_bytes | 41287680 | 39976960 … 42221568 | 5 |
| legacy | seconds | 2 | 2 … 2 | 5 |
| legacy | target_rate | 400 | 400 … 400 | 5 |
| legacy | viewers | 96 | 96 … 96 | 5 |
| legacy | worst_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | achieved_rate | 399.969377344547 | 399.948340072732 … 399.9854171316695 | 5 |
| adaptive_batch | affected | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_p99_us | 5519 | 4982 … 5910 | 5 |
| adaptive_batch | cold_rate | 100 | 100 … 100 | 5 |
| adaptive_batch | cold_received | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_samples | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_unique | 200 | 200 … 200 | 5 |
| adaptive_batch | cursor_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_count | 1000 | 1000 … 1000 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_sum | 0.005472525999999999 | 0.004999549000000003 … 0.006146392000000006 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="single"} | 80800 | 80800 … 80800 | 5 |
| adaptive_batch | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| adaptive_batch | duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | expected | 76800 | 76800 … 76800 | 5 |
| adaptive_batch | frames | 76800 | 76800 … 76800 | 5 |
| adaptive_batch | go_gc_duration_seconds_sum | 0.000532626 | 0.000382128 … 0.0008848339999999999 | 5 |
| adaptive_batch | go_goroutines | 0 | -1 … 1 | 5 |
| adaptive_batch | go_memstats_alloc_bytes_total | 19892776 | 19449776 … 21405600 | 5 |
| adaptive_batch | hot_p50_us | 1898 | 1869 … 2158 | 5 |
| adaptive_batch | hot_p99_us | 5775 | 4753 … 6196 | 5 |
| adaptive_batch | hot_samples | 6400 | 6400 … 6400 | 5 |
| adaptive_batch | missing | 0 | 0 … 0 | 5 |
| adaptive_batch | payload_bytes | 26376288 | 26373888 … 26379072 | 5 |
| adaptive_batch | process_cpu_seconds_total | 1.50575 | 1.466402 … 1.5262879999999999 | 5 |
| adaptive_batch | process_resident_memory_bytes | 16433152 | 15859712 … 17793024 | 5 |
| adaptive_batch | received | 76800 | 76800 … 76800 | 5 |
| adaptive_batch | rss_peak_bytes | 40747008 | 39993344 … 41861120 | 5 |
| adaptive_batch | seconds | 2 | 2 … 2 | 5 |
| adaptive_batch | target_rate | 400 | 400 … 400 | 5 |
| adaptive_batch | viewers | 96 | 96 … 96 | 5 |
| adaptive_batch | worst_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | achieved_rate | 399.94663212127637 | 399.9451825134218 … 399.9842422207935 | 5 |
| mixed_25pct_legacy | affected | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_p99_us | 5373 | 3436 … 13592 | 5 |
| mixed_25pct_legacy | cold_rate | 100 | 100 … 100 | 5 |
| mixed_25pct_legacy | cold_received | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_samples | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_unique | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_count | 1800 | 1800 … 1800 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_sum | 0.027345919 | 0.02017515499999999 … 0.03007777399999998 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="single"} | 80800 | 80800 … 80800 | 5 |
| mixed_25pct_legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | expected | 76800 | 76800 … 76800 | 5 |
| mixed_25pct_legacy | frames | 76800 | 76800 … 76800 | 5 |
| mixed_25pct_legacy | go_gc_duration_seconds_sum | 0.00048291800000000004 | 0.000395251 … 0.00065825 | 5 |
| mixed_25pct_legacy | go_goroutines | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | go_memstats_alloc_bytes_total | 20984752 | 16785904 … 23344960 | 5 |
| mixed_25pct_legacy | hot_p50_us | 1801 | 1441 … 1859 | 5 |
| mixed_25pct_legacy | hot_p99_us | 4808 | 4129 … 8847 | 5 |
| mixed_25pct_legacy | hot_samples | 6400 | 6400 … 6400 | 5 |
| mixed_25pct_legacy | missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | payload_bytes | 26376768 | 26374176 … 26379840 | 5 |
| mixed_25pct_legacy | process_cpu_seconds_total | 1.470658 | 1.328673 … 1.493655 | 5 |
| mixed_25pct_legacy | process_resident_memory_bytes | 16596992 | 15319040 … 17219584 | 5 |
| mixed_25pct_legacy | received | 76800 | 76800 … 76800 | 5 |
| mixed_25pct_legacy | rss_peak_bytes | 40599552 | 39796736 … 41320448 | 5 |
| mixed_25pct_legacy | seconds | 2 | 2 … 2 | 5 |
| mixed_25pct_legacy | target_rate | 400 | 400 … 400 | 5 |
| mixed_25pct_legacy | viewers | 96 | 96 … 96 | 5 |
| mixed_25pct_legacy | worst_missing | 0 | 0 … 0 | 5 |

## fanout_sweep high · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| legacy | achieved_rate | 699.9863837148669 | 699.8854125106321 … 699.9937916050636 | 5 |
| legacy | affected | 0 | 0 … 0 | 5 |
| legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| legacy | cold_gaps | 0 | 0 … 0 | 5 |
| legacy | cold_missing | 0 | 0 … 0 | 5 |
| legacy | cold_p99_us | 9390 | 4407 … 13966 | 5 |
| legacy | cold_rate | 100 | 100 … 100 | 5 |
| legacy | cold_received | 200 | 200 … 200 | 5 |
| legacy | cold_samples | 200 | 200 … 200 | 5 |
| legacy | cold_unique | 200 | 200 … 200 | 5 |
| legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_count | 1600 | 1600 … 1600 | 5 |
| legacy | danmaku_delivery_queue_wait_seconds_sum | 0.005895526999999998 | 0.0038570249999999935 … 0.011110376000000102 | 5 |
| legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| legacy | danmaku_fanout_frames_total{mode="single"} | 317600 | 317600 … 317600 | 5 |
| legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| legacy | duplicates | 0 | 0 … 0 | 5 |
| legacy | expected | 313600 | 313600 … 313600 | 5 |
| legacy | frames | 313600 | 313600 … 313600 | 5 |
| legacy | go_gc_duration_seconds_sum | 0.000446958 | 0.000355252 … 0.000548289 | 5 |
| legacy | go_goroutines | 0 | -1 … 0 | 5 |
| legacy | go_memstats_alloc_bytes_total | 27908008 | 26659144 … 30389488 | 5 |
| legacy | hot_p50_us | 1571 | 1367 … 1697 | 5 |
| legacy | hot_p99_us | 9273 | 4187 … 17800 | 5 |
| legacy | hot_samples | 11200 | 11200 … 11200 | 5 |
| legacy | missing | 0 | 0 … 0 | 5 |
| legacy | payload_bytes | 107823968 | 107819040 … 107831136 | 5 |
| legacy | process_cpu_seconds_total | 3.159016 | 3.0814500000000002 … 3.243673 | 5 |
| legacy | process_resident_memory_bytes | 25264128 | 23281664 … 26279936 | 5 |
| legacy | received | 313600 | 313600 … 313600 | 5 |
| legacy | rss_peak_bytes | 48988160 | 47628288 … 49823744 | 5 |
| legacy | seconds | 2 | 2 … 2 | 5 |
| legacy | target_rate | 700 | 700 … 700 | 5 |
| legacy | viewers | 224 | 224 … 224 | 5 |
| legacy | worst_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | achieved_rate | 699.9509346893907 | 699.9243394287728 … 699.9936603574164 | 5 |
| adaptive_batch | affected | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_missing | 0 | 0 … 0 | 5 |
| adaptive_batch | cold_p99_us | 3922 | 3136 … 5366 | 5 |
| adaptive_batch | cold_rate | 100 | 100 … 100 | 5 |
| adaptive_batch | cold_received | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_samples | 200 | 200 … 200 | 5 |
| adaptive_batch | cold_unique | 200 | 200 … 200 | 5 |
| adaptive_batch | cursor_gaps | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_count | 663 | 645 … 664 | 5 |
| adaptive_batch | danmaku_delivery_queue_wait_seconds_sum | 0.0015791249999999976 | 0.0008786519999999988 … 0.0019061959999999976 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="batch"} | 61152 | 60928 … 63616 | 5 |
| adaptive_batch | danmaku_fanout_frames_total{mode="single"} | 46560 | 40064 … 47008 | 5 |
| adaptive_batch | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| adaptive_batch | duplicates | 0 | 0 … 0 | 5 |
| adaptive_batch | expected | 313600 | 313600 … 313600 | 5 |
| adaptive_batch | frames | 103712 | 99680 … 103936 | 5 |
| adaptive_batch | go_gc_duration_seconds_sum | 0.000379085 | 0.000321044 … 0.000456375 | 5 |
| adaptive_batch | go_goroutines | 0 | -1 … 0 | 5 |
| adaptive_batch | go_memstats_alloc_bytes_total | 28100056 | 26843768 … 29452248 | 5 |
| adaptive_batch | hot_p50_us | 4807 | 4347 … 4870 | 5 |
| adaptive_batch | hot_p99_us | 9837 | 9356 … 11316 | 5 |
| adaptive_batch | hot_samples | 11200 | 11200 … 11200 | 5 |
| adaptive_batch | missing | 0 | 0 … 0 | 5 |
| adaptive_batch | payload_bytes | 108153024 | 108134880 … 108165120 | 5 |
| adaptive_batch | process_cpu_seconds_total | 1.547283 | 1.456989 … 1.608232 | 5 |
| adaptive_batch | process_resident_memory_bytes | 24428544 | 23724032 … 25034752 | 5 |
| adaptive_batch | received | 313600 | 313600 … 313600 | 5 |
| adaptive_batch | rss_peak_bytes | 48660480 | 47120384 … 49135616 | 5 |
| adaptive_batch | seconds | 2 | 2 … 2 | 5 |
| adaptive_batch | target_rate | 700 | 700 … 700 | 5 |
| adaptive_batch | viewers | 224 | 224 … 224 | 5 |
| adaptive_batch | worst_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | achieved_rate | 699.9775757183778 | 699.9225170275366 … 699.9900001428551 | 5 |
| mixed_25pct_legacy | affected | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | cold_p99_us | 8502 | 3302 … 25761 | 5 |
| mixed_25pct_legacy | cold_rate | 100 | 100 … 100 | 5 |
| mixed_25pct_legacy | cold_received | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_samples | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cold_unique | 200 | 200 … 200 | 5 |
| mixed_25pct_legacy | cursor_gaps | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_count | 2031 | 2023 … 2038 | 5 |
| mixed_25pct_legacy | danmaku_delivery_queue_wait_seconds_sum | 0.008194492999999985 | 0.005801187000000016 … 0.009281716999999974 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="batch"} | 48384 | 48048 … 49896 | 5 |
| mixed_25pct_legacy | danmaku_fanout_frames_total{mode="single"} | 105752 | 103736 … 107936 | 5 |
| mixed_25pct_legacy | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | duplicates | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | expected | 313600 | 313600 … 313600 | 5 |
| mixed_25pct_legacy | frames | 150808 | 149464 … 151984 | 5 |
| mixed_25pct_legacy | go_gc_duration_seconds_sum | 0.000499165 | 0.00044649899999999994 … 0.001170501 | 5 |
| mixed_25pct_legacy | go_goroutines | 0 | 0 … 2 | 5 |
| mixed_25pct_legacy | go_memstats_alloc_bytes_total | 28841576 | 27893408 … 30083256 | 5 |
| mixed_25pct_legacy | hot_p50_us | 3411 | 3263 … 3520 | 5 |
| mixed_25pct_legacy | hot_p99_us | 11451 | 8426 … 27469 | 5 |
| mixed_25pct_legacy | hot_samples | 11200 | 11200 … 11200 | 5 |
| mixed_25pct_legacy | missing | 0 | 0 … 0 | 5 |
| mixed_25pct_legacy | payload_bytes | 108077872 | 108061072 … 108090920 | 5 |
| mixed_25pct_legacy | process_cpu_seconds_total | 1.97927 | 1.8948429999999998 … 2.004198 | 5 |
| mixed_25pct_legacy | process_resident_memory_bytes | 24608768 | 23625728 … 26247168 | 5 |
| mixed_25pct_legacy | received | 313600 | 313600 … 313600 | 5 |
| mixed_25pct_legacy | rss_peak_bytes | 48873472 | 47054848 … 50544640 | 5 |
| mixed_25pct_legacy | seconds | 2 | 2 … 2 | 5 |
| mixed_25pct_legacy | target_rate | 700 | 700 … 700 | 5 |
| mixed_25pct_legacy | viewers | 224 | 224 … 224 | 5 |
| mixed_25pct_legacy | worst_missing | 0 | 0 … 0 | 5 |

## isolation_stalled · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| baseline_single_shard_unbounded | achieved_rate | 799.9744008191738 | 799.9671013529568 … 799.9919668806644 | 5 |
| baseline_single_shard_unbounded | affected | 0 | 0 … 0 | 5 |
| baseline_single_shard_unbounded | cold_duplicates | 0 | 0 … 0 | 5 |
| baseline_single_shard_unbounded | cold_gaps | 0 | 0 … 0 | 5 |
| baseline_single_shard_unbounded | cold_missing | 0 | 0 … 0 | 5 |
| baseline_single_shard_unbounded | cold_p99_us | 8902 | 4386 … 23031 | 5 |
| baseline_single_shard_unbounded | cold_rate | 100 | 100 … 100 | 5 |
| baseline_single_shard_unbounded | cold_received | 200 | 200 … 200 | 5 |
| baseline_single_shard_unbounded | cold_samples | 200 | 200 … 200 | 5 |
| baseline_single_shard_unbounded | cold_unique | 200 | 200 … 200 | 5 |
| baseline_single_shard_unbounded | cursor_gaps | 0 | 0 … 0 | 5 |
| baseline_single_shard_unbounded | danmaku_delivery_queue_wait_seconds_count | 2225 | 2208 … 2238 | 5 |
| baseline_single_shard_unbounded | danmaku_delivery_queue_wait_seconds_sum | 0.009753839999999995 | 0.009079964000000005 … 0.011944197999999961 | 5 |
| baseline_single_shard_unbounded | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| baseline_single_shard_unbounded | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| baseline_single_shard_unbounded | danmaku_fanout_frames_total{mode="batch"} | 49600 | 49440 … 50400 | 5 |
| baseline_single_shard_unbounded | danmaku_fanout_frames_total{mode="single"} | 41602 | 38081 … 43849 | 5 |
| baseline_single_shard_unbounded | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| baseline_single_shard_unbounded | duplicates | 0 | 0 … 0 | 5 |
| baseline_single_shard_unbounded | expected | 256000 | 256000 … 256000 | 5 |
| baseline_single_shard_unbounded | frames | 68000 | 65280 … 70080 | 5 |
| baseline_single_shard_unbounded | go_gc_duration_seconds_sum | 0.00046903999999999994 | 0.00042175 … 0.000572626 | 5 |
| baseline_single_shard_unbounded | go_goroutines | 0 | -1 … 0 | 5 |
| baseline_single_shard_unbounded | go_memstats_alloc_bytes_total | 47292504 | 45187152 … 48799928 | 5 |
| baseline_single_shard_unbounded | hot_p50_us | 4499 | 4369 … 4642 | 5 |
| baseline_single_shard_unbounded | hot_p99_us | 11077 | 9850 … 24154 | 5 |
| baseline_single_shard_unbounded | hot_samples | 12800 | 12800 … 12800 | 5 |
| baseline_single_shard_unbounded | missing | 0 | 0 … 0 | 5 |
| baseline_single_shard_unbounded | payload_bytes | 88336160 | 88315360 … 88346720 | 5 |
| baseline_single_shard_unbounded | process_cpu_seconds_total | 1.642194 | 1.5075109999999998 … 1.7136779999999998 | 5 |
| baseline_single_shard_unbounded | process_resident_memory_bytes | 46874624 | 44253184 … 47513600 | 5 |
| baseline_single_shard_unbounded | received | 256000 | 256000 … 256000 | 5 |
| baseline_single_shard_unbounded | rss_peak_bytes | 70189056 | 67862528 … 71532544 | 5 |
| baseline_single_shard_unbounded | seconds | 2 | 2 … 2 | 5 |
| baseline_single_shard_unbounded | target_rate | 800 | 800 … 800 | 5 |
| baseline_single_shard_unbounded | viewers | 160 | 160 … 160 | 5 |
| baseline_single_shard_unbounded | worst_missing | 0 | 0 … 0 | 5 |
| protected_default | achieved_rate | 799.9865002278086 | 799.9698847337042 … 799.997116810391 | 5 |
| protected_default | affected | 0 | 0 … 0 | 5 |
| protected_default | cold_duplicates | 0 | 0 … 0 | 5 |
| protected_default | cold_gaps | 0 | 0 … 0 | 5 |
| protected_default | cold_missing | 0 | 0 … 0 | 5 |
| protected_default | cold_p99_us | 6086 | 4634 … 8762 | 5 |
| protected_default | cold_rate | 100 | 100 … 100 | 5 |
| protected_default | cold_received | 200 | 200 … 200 | 5 |
| protected_default | cold_samples | 200 | 200 … 200 | 5 |
| protected_default | cold_unique | 200 | 200 … 200 | 5 |
| protected_default | cursor_gaps | 0 | 0 … 0 | 5 |
| protected_default | danmaku_delivery_queue_wait_seconds_count | 2222 | 2216 … 2235 | 5 |
| protected_default | danmaku_delivery_queue_wait_seconds_sum | 0.008627655999999992 | 0.005915971000000013 … 0.012640569999999966 | 5 |
| protected_default | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| protected_default | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| protected_default | danmaku_fanout_frames_total{mode="batch"} | 50240 | 49600 … 51840 | 5 |
| protected_default | danmaku_fanout_frames_total{mode="single"} | 40487 | 39041 … 43204 | 5 |
| protected_default | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| protected_default | duplicates | 0 | 0 … 0 | 5 |
| protected_default | expected | 256000 | 256000 … 256000 | 5 |
| protected_default | frames | 67520 | 66560 … 69600 | 5 |
| protected_default | go_gc_duration_seconds_sum | 0.000530291 | 0.000323166 … 0.000642334 | 5 |
| protected_default | go_goroutines | 0 | -1 … 1 | 5 |
| protected_default | go_memstats_alloc_bytes_total | 31460112 | 29480056 … 33047456 | 5 |
| protected_default | hot_p50_us | 4563 | 4523 … 4610 | 5 |
| protected_default | hot_p99_us | 10712 | 9432 … 13025 | 5 |
| protected_default | hot_samples | 12800 | 12800 … 12800 | 5 |
| protected_default | missing | 0 | 0 … 0 | 5 |
| protected_default | payload_bytes | 88330720 | 88304960 … 88341440 | 5 |
| protected_default | process_cpu_seconds_total | 1.571771 | 1.505649 … 1.651275 | 5 |
| protected_default | process_resident_memory_bytes | 24363008 | 23134208 … 24821760 | 5 |
| protected_default | received | 256000 | 256000 … 256000 | 5 |
| protected_default | rss_peak_bytes | 48103424 | 47349760 … 48660480 | 5 |
| protected_default | seconds | 2 | 2 … 2 | 5 |
| protected_default | target_rate | 800 | 800 … 800 | 5 |
| protected_default | viewers | 160 | 160 … 160 | 5 |
| protected_default | worst_missing | 0 | 0 … 0 | 5 |

## migration_strategies · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| crash | complete_viewers | 144 | 144 … 144 | 5 |
| crash | connect_peak_qps | 310 | 270 … 350 | 5 |
| crash | danmaku_delivery_queue_wait_seconds_count | 24 | 24 … 24 | 5 |
| crash | danmaku_delivery_queue_wait_seconds_sum | 0.000171127 | 0.000129083 … 0.000182377 | 5 |
| crash | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| crash | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| crash | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| crash | danmaku_fanout_frames_total{mode="single"} | 3214 | 3208 … 3225 | 5 |
| crash | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| crash | drain_ms | 454 | 445 … 456 | 5 |
| crash | events | 24 | 24 … 24 | 5 |
| crash | go_gc_duration_seconds_sum | 0.000885791 | 0.000331918 … 0.001350583 | 5 |
| crash | go_goroutines | 6 | 5 … 6 | 5 |
| crash | go_memstats_alloc_bytes_total | 8276832 | 7425688 … 8479424 | 5 |
| crash | invited | 0 | 0 … 0 | 5 |
| crash | last_handoff_ms | 454 | 445 … 456 | 5 |
| crash | migrating_delivery_p50_us | 4356 | 4200 … 4449 | 5 |
| crash | migrating_delivery_p95_us | 169787 | 161924 … 182740 | 5 |
| crash | migrating_delivery_p99_us | 403970 | 378716 … 416493 | 5 |
| crash | migrating_delivery_samples | 1584 | 1584 … 1584 | 5 |
| crash | moved | 72 | 72 … 72 | 5 |
| crash | peak_attempts_per_sec | 72 | 72 … 72 | 5 |
| crash | process_cpu_seconds_total | 0.22381299999999998 | 0.205347 … 0.243033 | 5 |
| crash | process_resident_memory_bytes | 11599872 | 7503872 … 11829248 | 5 |
| crash | recent_peak_qps | 300 | 270 … 350 | 5 |
| crash | recovery_p50_ms | 299 | 265 … 325 | 5 |
| crash | recovery_p95_ms | 445 | 426 … 449 | 5 |
| crash | recovery_p99_ms | 450 | 440 … 451 | 5 |
| crash | recovery_samples | 72 | 72 … 72 | 5 |
| crash | resident_delivery_p50_us | 4241 | 4149 … 4333 | 5 |
| crash | resident_delivery_p95_us | 5345 | 5089 … 5588 | 5 |
| crash | resident_delivery_p99_ms | 5 | 5 … 7 | 5 |
| crash | resident_delivery_p99_us | 5978 | 5279 … 7012 | 5 |
| crash | resident_delivery_samples | 1584 | 1584 … 1584 | 5 |
| crash | resident_during_handoff_p50_us | 2575 | 1632 … 3704 | 5 |
| crash | resident_during_handoff_p95_us | 4641 | 1758 … 5509 | 5 |
| crash | resident_during_handoff_p99_us | 4716 | 1854 … 5590 | 5 |
| crash | resident_during_handoff_samples | 144 | 144 … 144 | 5 |
| crash | source_exit_ms | 5.157 | 2.614 … 5.79 | 5 |
| crash | viewers | 144 | 144 … 144 | 5 |
| fixed | complete_viewers | 144 | 144 … 144 | 5 |
| fixed | connect_peak_qps | 450 | 240 … 480 | 5 |
| fixed | danmaku_delivery_queue_wait_seconds_count | 24 | 24 … 24 | 5 |
| fixed | danmaku_delivery_queue_wait_seconds_sum | 0.000147252 | 0.00014370699999999997 … 0.00016249700000000005 | 5 |
| fixed | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| fixed | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| fixed | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| fixed | danmaku_fanout_frames_total{mode="single"} | 3240 | 3240 … 3240 | 5 |
| fixed | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| fixed | drain_ms | 407.609 | 404.541 … 409.234 | 5 |
| fixed | events | 24 | 24 … 24 | 5 |
| fixed | go_gc_duration_seconds_sum | 0.001164499 | 0.000627333 … 0.002250168 | 5 |
| fixed | go_goroutines | 24 | 23 … 24 | 5 |
| fixed | go_memstats_alloc_bytes_total | 8406760 | 7356632 … 9329032 | 5 |
| fixed | invited | 72 | 72 … 72 | 5 |
| fixed | last_handoff_ms | 316 | 306 … 317 | 5 |
| fixed | migrating_delivery_p50_us | 4267 | 4022 … 4534 | 5 |
| fixed | migrating_delivery_p95_us | 5523 | 5088 … 5971 | 5 |
| fixed | migrating_delivery_p99_us | 6332 | 5448 … 10142 | 5 |
| fixed | migrating_delivery_samples | 1584 | 1584 … 1584 | 5 |
| fixed | moved | 72 | 72 … 72 | 5 |
| fixed | peak_attempts_per_sec | 72 | 72 … 72 | 5 |
| fixed | process_cpu_seconds_total | 0.184971 | 0.175184 … 0.188064 | 5 |
| fixed | process_resident_memory_bytes | 12468224 | 12140544 … 12779520 | 5 |
| fixed | recent_peak_qps | 480 | 240 … 480 | 5 |
| fixed | recovery_p50_ms | 12 | 5 … 17 | 5 |
| fixed | recovery_p95_ms | 14 | 8 … 20 | 5 |
| fixed | recovery_p99_ms | 15 | 8 … 20 | 5 |
| fixed | recovery_samples | 72 | 72 … 72 | 5 |
| fixed | resident_delivery_p50_us | 4305 | 4044 … 4495 | 5 |
| fixed | resident_delivery_p95_us | 5493 | 4958 … 5918 | 5 |
| fixed | resident_delivery_p99_ms | 6 | 5 … 9 | 5 |
| fixed | resident_delivery_p99_us | 6266 | 5344 … 9553 | 5 |
| fixed | resident_delivery_samples | 1584 | 1584 … 1584 | 5 |
| fixed | resident_during_handoff_p50_us | 4463 | 1945 … 4867 | 5 |
| fixed | resident_during_handoff_p95_us | 5341 | 5005 … 9591 | 5 |
| fixed | resident_during_handoff_p99_us | 5374 | 5067 … 9634 | 5 |
| fixed | resident_during_handoff_samples | 144 | 144 … 144 | 5 |
| fixed | source_exit_ms | 407.609 | 404.541 … 409.234 | 5 |
| fixed | viewers | 144 | 144 … 144 | 5 |
| bounded | complete_viewers | 144 | 144 … 144 | 5 |
| bounded | connect_peak_qps | 80 | 80 … 80 | 5 |
| bounded | danmaku_delivery_queue_wait_seconds_count | 24 | 24 … 24 | 5 |
| bounded | danmaku_delivery_queue_wait_seconds_sum | 0.00014533799999999996 | 0.00011216599999999997 … 0.00015945499999999997 | 5 |
| bounded | danmaku_dropped_messages_total{reason="client_full"} | 0 | 0 … 0 | 5 |
| bounded | danmaku_dropped_messages_total{reason="shard_full"} | 0 | 0 … 0 | 5 |
| bounded | danmaku_fanout_frames_total{mode="batch"} | 0 | 0 … 0 | 5 |
| bounded | danmaku_fanout_frames_total{mode="single"} | 3026 | 3020 … 3028 | 5 |
| bounded | danmaku_kicked_slow_clients_total | 0 | 0 … 0 | 5 |
| bounded | drain_ms | 1905.344 | 1903.243 … 1907.242 | 5 |
| bounded | events | 24 | 24 … 24 | 5 |
| bounded | go_gc_duration_seconds_sum | 0.0007021239999999999 | 0.000359458 … 0.002040291 | 5 |
| bounded | go_goroutines | 4 | 4 … 5 | 5 |
| bounded | go_memstats_alloc_bytes_total | 8033744 | 7727712 … 8955960 | 5 |
| bounded | invited | 72 | 72 … 72 | 5 |
| bounded | last_handoff_ms | 1811 | 1810 … 1812 | 5 |
| bounded | migrating_delivery_p50_us | 4423 | 4250 … 4526 | 5 |
| bounded | migrating_delivery_p95_us | 5506 | 5189 … 6180 | 5 |
| bounded | migrating_delivery_p99_us | 6484 | 5601 … 8155 | 5 |
| bounded | migrating_delivery_samples | 1584 | 1584 … 1584 | 5 |
| bounded | moved | 72 | 72 … 72 | 5 |
| bounded | peak_attempts_per_sec | 36 | 36 … 36 | 5 |
| bounded | process_cpu_seconds_total | 0.213808 | 0.195677 … 0.227207 | 5 |
| bounded | process_resident_memory_bytes | 11354112 | 10911744 … 12746752 | 5 |
| bounded | recent_peak_qps | 80 | 80 … 80 | 5 |
| bounded | recovery_p50_ms | 8 | 8 … 8 | 5 |
| bounded | recovery_p95_ms | 11 | 10 … 17 | 5 |
| bounded | recovery_p99_ms | 11 | 11 … 18 | 5 |
| bounded | recovery_samples | 72 | 72 … 72 | 5 |
| bounded | resident_delivery_p50_us | 4456 | 4213 … 4511 | 5 |
| bounded | resident_delivery_p95_us | 5346 | 5197 … 6128 | 5 |
| bounded | resident_delivery_p99_ms | 6 | 5 … 7 | 5 |
| bounded | resident_delivery_p99_us | 6128 | 5424 … 7362 | 5 |
| bounded | resident_delivery_samples | 1584 | 1584 … 1584 | 5 |
| bounded | resident_during_handoff_p50_us | 4539 | 3621 … 4637 | 5 |
| bounded | resident_during_handoff_p95_us | 5861 | 5206 … 7223 | 5 |
| bounded | resident_during_handoff_p99_us | 6225 | 5361 … 8840 | 5 |
| bounded | resident_during_handoff_samples | 576 | 576 … 576 | 5 |
| bounded | source_exit_ms | 1905.344 | 1903.243 … 1907.242 | 5 |
| bounded | viewers | 144 | 144 … 144 | 5 |

## history_pagination page_1 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| offset_baseline | p50_us | 518.583 | 487.791 … 554.333 | 5 |
| offset_baseline | p95_us | 576.625 | 532.5 … 1625.75 | 5 |
| offset_baseline | p999_us | 576.625 | 532.5 … 1625.75 | 5 |
| offset_baseline | p99_us | 576.625 | 532.5 … 1625.75 | 5 |
| offset_baseline | samples | 6 | 6 … 6 | 5 |
| cursor_before_id_prod | p50_us | 514.333 | 474.584 … 522.292 | 5 |
| cursor_before_id_prod | p95_us | 570.834 | 511.917 … 1608.041 | 5 |
| cursor_before_id_prod | p999_us | 570.834 | 511.917 … 1608.041 | 5 |
| cursor_before_id_prod | p99_us | 570.834 | 511.917 … 1608.041 | 5 |
| cursor_before_id_prod | samples | 6 | 6 … 6 | 5 |

## history_pagination page_200 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| offset_baseline | p50_us | 9178.834 | 9076.083 … 9355.459 | 5 |
| offset_baseline | p95_us | 9706.541 | 9387 … 9818.375 | 5 |
| offset_baseline | p999_us | 9706.541 | 9387 … 9818.375 | 5 |
| offset_baseline | p99_us | 9706.541 | 9387 … 9818.375 | 5 |
| offset_baseline | samples | 6 | 6 … 6 | 5 |
| cursor_before_id_prod | p50_us | 533.834 | 493.375 … 564.666 | 5 |
| cursor_before_id_prod | p95_us | 565.75 | 497.042 … 613.375 | 5 |
| cursor_before_id_prod | p999_us | 565.75 | 497.042 … 613.375 | 5 |
| cursor_before_id_prod | p99_us | 565.75 | 497.042 … 613.375 | 5 |
| cursor_before_id_prod | samples | 6 | 6 … 6 | 5 |

## history_pagination page_1000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| offset_baseline | p50_us | 37609.5 | 37261.5 … 40815.708 | 5 |
| offset_baseline | p95_us | 40599.041 | 38220.666 … 46755.792 | 5 |
| offset_baseline | p999_us | 40599.041 | 38220.666 … 46755.792 | 5 |
| offset_baseline | p99_us | 40599.041 | 38220.666 … 46755.792 | 5 |
| offset_baseline | samples | 6 | 6 … 6 | 5 |
| cursor_before_id_prod | p50_us | 523.292 | 505.5 … 560.042 | 5 |
| cursor_before_id_prod | p95_us | 676.625 | 617.333 … 719.708 | 5 |
| cursor_before_id_prod | p999_us | 676.625 | 617.333 … 719.708 | 5 |
| cursor_before_id_prod | p99_us | 676.625 | 617.333 … 719.708 | 5 |
| cursor_before_id_prod | samples | 6 | 6 … 6 | 5 |

## history_pagination page_2000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| offset_baseline | p50_us | 90843.583 | 89905.083 … 91069.625 | 5 |
| offset_baseline | p95_us | 92626.375 | 90400.583 … 103788.208 | 5 |
| offset_baseline | p999_us | 92626.375 | 90400.583 … 103788.208 | 5 |
| offset_baseline | p99_us | 92626.375 | 90400.583 … 103788.208 | 5 |
| offset_baseline | samples | 6 | 6 … 6 | 5 |
| cursor_before_id_prod | p50_us | 581.542 | 562.041 … 584.666 | 5 |
| cursor_before_id_prod | p95_us | 685.291 | 664.042 … 748.375 | 5 |
| cursor_before_id_prod | p999_us | 685.291 | 664.042 … 748.375 | 5 |
| cursor_before_id_prod | p99_us | 685.291 | 664.042 … 748.375 | 5 |
| cursor_before_id_prod | samples | 6 | 6 … 6 | 5 |

## history_pagination page_3600 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| offset_baseline | p50_us | 162105.792 | 160594.042 … 162483.666 | 5 |
| offset_baseline | p95_us | 164869.75 | 163672.583 … 171924.167 | 5 |
| offset_baseline | p999_us | 164869.75 | 163672.583 … 171924.167 | 5 |
| offset_baseline | p99_us | 164869.75 | 163672.583 … 171924.167 | 5 |
| offset_baseline | samples | 6 | 6 … 6 | 5 |
| cursor_before_id_prod | p50_us | 552.792 | 526.416 … 621.667 | 5 |
| cursor_before_id_prod | p95_us | 689.042 | 625.083 … 1152.042 | 5 |
| cursor_before_id_prod | p999_us | 689.042 | 625.083 … 1152.042 | 5 |
| cursor_before_id_prod | p99_us | 689.042 | 625.083 … 1152.042 | 5 |
| cursor_before_id_prod | samples | 6 | 6 … 6 | 5 |

## pg_archive_write batch_128 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| row_by_row_tx | alloc_bytes_per_batch | 72319.04 | 72296 … 72319.04 | 5 |
| row_by_row_tx | allocs_per_batch | 1676.01 | 1676 … 1676.01 | 5 |
| row_by_row_tx | batch_p50_us | 28437.167 | 28228.75 … 28530.459 | 5 |
| row_by_row_tx | batch_p95_us | 32576.208 | 30611.166 … 33801.333 | 5 |
| row_by_row_tx | batch_p999_us | 43757.042 | 36849.25 … 50787.875 | 5 |
| row_by_row_tx | batch_p99_us | 35871.75 | 35461.167 … 39687.125 | 5 |
| row_by_row_tx | batch_samples | 100 | 100 … 100 | 5 |
| row_by_row_tx | duplicate_rows | 0 | 0 … 0 | 5 |
| row_by_row_tx | rows_per_second | 4489.995356916848 | 4433.35432393836 … 4539.808737098907 | 5 |
| multivalue_insert | alloc_bytes_per_batch | 159343.92 | 159323.92 … 159361.12 | 5 |
| multivalue_insert | allocs_per_batch | 1328.28 | 1328.21 … 1328.35 | 5 |
| multivalue_insert | batch_p50_us | 1524.25 | 1465.708 … 1550.25 | 5 |
| multivalue_insert | batch_p95_us | 2076.084 | 1851.583 … 2446.792 | 5 |
| multivalue_insert | batch_p999_us | 2691.666 | 2605.625 … 6033.542 | 5 |
| multivalue_insert | batch_p99_us | 2527.458 | 2465.125 … 5896.625 | 5 |
| multivalue_insert | batch_samples | 100 | 100 … 100 | 5 |
| multivalue_insert | duplicate_rows | 0 | 0 … 0 | 5 |
| multivalue_insert | rows_per_second | 80253.73975373701 | 76539.11642167672 … 82586.70595987287 | 5 |
| unnest_prod | alloc_bytes_per_batch | 73960.8 | 73937.6 … 73961.12 | 5 |
| unnest_prod | allocs_per_batch | 809.94 | 809.93 … 809.95 | 5 |
| unnest_prod | batch_p50_us | 1541.916 | 1536.25 … 1610.5 | 5 |
| unnest_prod | batch_p95_us | 2007.667 | 1844 … 2595.458 | 5 |
| unnest_prod | batch_p999_us | 2940.958 | 2599.084 … 4508.709 | 5 |
| unnest_prod | batch_p99_us | 2576.625 | 2554.084 … 3901.375 | 5 |
| unnest_prod | batch_samples | 100 | 100 … 100 | 5 |
| unnest_prod | duplicate_rows | 0 | 0 … 0 | 5 |
| unnest_prod | rows_per_second | 80562.93349781599 | 72753.10834813499 … 81638.98789263834 | 5 |

## redis_lua_atomicity 0s · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| multi_cmd_baseline | extra_broadcasts | 35 | 35 … 35 | 5 |
| multi_cmd_baseline | p50_us | 3414.625 | 2683 … 3589.459 | 5 |
| multi_cmd_baseline | p95_us | 3743.5 | 3105.834 … 3976.333 | 5 |
| multi_cmd_baseline | p999_us | 3834.083 | 3329.833 … 6998.833 | 5 |
| multi_cmd_baseline | p99_us | 3786.833 | 3270.666 … 4634.541 | 5 |
| multi_cmd_baseline | qps | 295.03496717551593 | 280.0920711934217 … 364.4658003261138 | 5 |
| multi_cmd_baseline | race_duplicate_rate | 0.21875 | 0.21875 … 0.21875 | 5 |
| multi_cmd_baseline | samples | 100 | 100 … 100 | 5 |
| multi_cmd_baseline | stream_writes | 40 | 40 … 40 | 5 |
| lua_evalsha_prod | extra_broadcasts | 0 | 0 … 0 | 5 |
| lua_evalsha_prod | p50_us | 386.417 | 363.833 … 432.833 | 5 |
| lua_evalsha_prod | p95_us | 413.417 | 404.791 … 474.083 | 5 |
| lua_evalsha_prod | p999_us | 451.417 | 429 … 676.583 | 5 |
| lua_evalsha_prod | p99_us | 431.583 | 411.209 … 494.125 | 5 |
| lua_evalsha_prod | qps | 2586.0954492142187 | 2322.594916383215 … 2778.8230011738583 | 5 |
| lua_evalsha_prod | race_duplicate_rate | 0 | 0 … 0 | 5 |
| lua_evalsha_prod | samples | 100 | 100 … 100 | 5 |
| lua_evalsha_prod | stream_writes | 5 | 5 … 5 | 5 |

## redis_lua_atomicity 2ms · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| multi_cmd_baseline | extra_broadcasts | 35 | 35 … 35 | 5 |
| multi_cmd_baseline | p50_us | 34824 | 32998.708 … 35438.333 | 5 |
| multi_cmd_baseline | p95_us | 38237.666 | 36997.708 … 39564.792 | 5 |
| multi_cmd_baseline | p999_us | 42523.25 | 38199.917 … 49841.75 | 5 |
| multi_cmd_baseline | p99_us | 40920.5 | 37707.875 … 43755 | 5 |
| multi_cmd_baseline | qps | 29.47157611675259 | 28.420754924171117 … 31.016260972680797 | 5 |
| multi_cmd_baseline | race_duplicate_rate | 0.21875 | 0.21875 … 0.21875 | 5 |
| multi_cmd_baseline | samples | 100 | 100 … 100 | 5 |
| multi_cmd_baseline | stream_writes | 40 | 40 … 40 | 5 |
| lua_evalsha_prod | extra_broadcasts | 0 | 0 … 0 | 5 |
| lua_evalsha_prod | p50_us | 3643.625 | 3535.958 … 4147.209 | 5 |
| lua_evalsha_prod | p95_us | 4769.083 | 4380.458 … 5109.625 | 5 |
| lua_evalsha_prod | p999_us | 6221.458 | 5399.333 … 11693.25 | 5 |
| lua_evalsha_prod | p99_us | 5156 | 4783.375 … 8982.75 | 5 |
| lua_evalsha_prod | qps | 264.74762900495386 | 232.05575716920714 … 273.0089059845974 | 5 |
| lua_evalsha_prod | race_duplicate_rate | 0 | 0 … 0 | 5 |
| lua_evalsha_prod | samples | 100 | 100 … 100 | 5 |
| lua_evalsha_prod | stream_writes | 5 | 5 … 5 | 5 |

## sensitive_filter_scaling words_100 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| naive_contains | hit_us | 0.95916 | 0.84541 … 1.08875 | 5 |
| naive_contains | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| naive_contains | miss_us | 0.70041 | 0.64125 … 0.81916 | 5 |
| merged_regexp | hit_us | 96.38708000000001 | 95.7625 … 102.4375 | 5 |
| merged_regexp | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| merged_regexp | miss_us | 96.38958000000001 | 95.81458 … 108.42165999999999 | 5 |
| ac_prod | hit_us | 1.41333 | 1.33209 … 1.5379099999999999 | 5 |
| ac_prod | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| ac_prod | miss_us | 1.35125 | 1.28458 … 1.45333 | 5 |

## sensitive_filter_scaling words_1000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| naive_contains | hit_us | 9.11083 | 8.89291 … 9.32958 | 5 |
| naive_contains | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| naive_contains | miss_us | 6.5804100000000005 | 6.34792 … 6.93208 | 5 |
| merged_regexp | hit_us | 1518.8816699999998 | 1467.3204199999998 … 1550.9966699999998 | 5 |
| merged_regexp | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| merged_regexp | miss_us | 1475.76959 | 1454.82542 … 1627.75 | 5 |
| ac_prod | hit_us | 1.42541 | 1.4125 … 1.47042 | 5 |
| ac_prod | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| ac_prod | miss_us | 1.36625 | 1.30083 … 1.41875 | 5 |

## sensitive_filter_scaling words_5000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| naive_contains | hit_us | 46.90459 | 45.715 … 46.98708 | 5 |
| naive_contains | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| naive_contains | miss_us | 34.42792 | 33.77708 … 34.79125 | 5 |
| ac_prod | hit_us | 1.47083 | 1.39459 … 1.5616700000000001 | 5 |
| ac_prod | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| ac_prod | miss_us | 1.41792 | 1.28917 … 1.46584 | 5 |

## sensitive_filter_scaling words_20000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| naive_contains | hit_us | 184.46666000000002 | 182.27375 … 186.51292 | 5 |
| naive_contains | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| naive_contains | miss_us | 137.23083 | 133.95125 … 138.78041 | 5 |
| ac_prod | hit_us | 1.3975 | 1.34208 … 1.59333 | 5 |
| ac_prod | miss_allocs_per_op | 0 | 0 … 0 | 5 |
| ac_prod | miss_us | 1.31166 | 1.26333 … 1.59209 | 5 |

## sensitive_filter_scaling reload_5000 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| rwmutex_rebuild_baseline | reload_p50_us | 2 | 2 … 2.041 | 5 |
| rwmutex_rebuild_baseline | reload_p95_us | 13829.292 | 11456.375 … 14249.625 | 5 |
| rwmutex_rebuild_baseline | reload_p999_us | 33240.541 | 25598.042 … 44595.959 | 5 |
| rwmutex_rebuild_baseline | reload_p99_us | 23408.709 | 18758.5 … 30666.041 | 5 |
| rwmutex_rebuild_baseline | reload_samples | 160 | 160 … 160 | 5 |
| atomic_pointer_prod | reload_p50_us | 1.625 | 1.541 … 1.917 | 5 |
| atomic_pointer_prod | reload_p95_us | 4.417 | 3.459 … 4.5 | 5 |
| atomic_pointer_prod | reload_p999_us | 82.166 | 80.583 … 88.334 | 5 |
| atomic_pointer_prod | reload_p99_us | 9.083 | 6.75 … 9.958 | 5 |
| atomic_pointer_prod | reload_samples | 322356 | 281622 … 360931 | 5 |

## live_source_singleflight burst_120 · PASS

| Arm | Metric | Median | 95% CI | Runs |
| --- | --- | --- | --- | --- |
| direct_no_coalesce | cold_p50_us | 86699.333 | 84901.333 … 89937.625 | 5 |
| direct_no_coalesce | cold_p95_us | 89352.083 | 86294.5 … 92441.042 | 5 |
| direct_no_coalesce | cold_p999_us | 89501.25 | 86779.834 … 92522.167 | 5 |
| direct_no_coalesce | cold_p99_us | 89470.25 | 86352.208 … 92488 | 5 |
| direct_no_coalesce | cold_samples | 120 | 120 … 120 | 5 |
| direct_no_coalesce | cold_upstream_calls | 120 | 120 … 120 | 5 |
| direct_no_coalesce | warm_p50_us | 84151.75 | 83010.708 … 86693.792 | 5 |
| direct_no_coalesce | warm_p95_us | 85954.708 | 85460.208 … 87828.583 | 5 |
| direct_no_coalesce | warm_p999_us | 86545.959 | 86202.084 … 88546.375 | 5 |
| direct_no_coalesce | warm_p99_us | 86315.417 | 86107.333 … 88283.083 | 5 |
| direct_no_coalesce | warm_samples | 120 | 120 … 120 | 5 |
| direct_no_coalesce | warm_upstream_calls | 120 | 120 … 120 | 5 |
| prod_singleflight_cache | cold_p50_us | 82283.417 | 81215.125 … 83029.125 | 5 |
| prod_singleflight_cache | cold_p95_us | 82447.583 | 81370.5 … 83131.208 | 5 |
| prod_singleflight_cache | cold_p999_us | 82488 | 81388.542 … 83185.875 | 5 |
| prod_singleflight_cache | cold_p99_us | 82461.75 | 81381.292 … 83160.25 | 5 |
| prod_singleflight_cache | cold_samples | 120 | 120 … 120 | 5 |
| prod_singleflight_cache | cold_upstream_calls | 1 | 1 … 1 | 5 |
| prod_singleflight_cache | warm_p50_us | 0.5 | 0.333 … 0.75 | 5 |
| prod_singleflight_cache | warm_p95_us | 3.083 | 1.708 … 16.542 | 5 |
| prod_singleflight_cache | warm_p999_us | 7.542 | 5.375 … 27.167 | 5 |
| prod_singleflight_cache | warm_p99_us | 7 | 4.583 … 26.166 | 5 |
| prod_singleflight_cache | warm_samples | 120 | 120 … 120 | 5 |
| prod_singleflight_cache | warm_upstream_calls | 0 | 0 … 0 | 5 |

