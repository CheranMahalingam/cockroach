#!/bin/bash

num_allocs=$(cat flush | grep "ALLOC" | wc -l)
avg_bytes=$(($(cat flush | grep "ALLOC" | awk '{SUM+=$3}END{print SUM}')/$num_allocs))

num_64k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 57344 && $3 <= 65536) print $3}' | wc -l)
num_56k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 49152 && $3 <= 57344) print $3}' | wc -l)
num_48k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 40960 && $3 <= 49152) print $3}' | wc -l)
num_40k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 32768 && $3 <= 40960) print $3}' | wc -l)
num_32k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 28672 && $3 <= 32768) print $3}' | wc -l)
num_28k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 24576 && $3 <= 28672) print $3}' | wc -l)
num_24k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 20480 && $3 <= 24576) print $3}' | wc -l)
num_20k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 16384 && $3 <= 20480) print $3}' | wc -l)
num_16k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 14336 && $3 <= 16384) print $3}' | wc -l)

frag_bytes_64k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 57344 && $3 <= 65536) SUM+=(65536 - $3)}END{print SUM}')
frag_bytes_56k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 49152 && $3 <= 57344) SUM+=(57344 - $3)}END{print SUM}')
frag_bytes_48k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 40960 && $3 <= 49152) SUM+=(49152 - $3)}END{print SUM}')
frag_bytes_40k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 32768 && $3 <= 40960) SUM+=(40960 - $3)}END{print SUM}')
frag_bytes_32k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 28672 && $3 <= 32768) SUM+=(32768 - $3)}END{print SUM}')
frag_bytes_28k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 24576 && $3 <= 28672) SUM+=(28672- $3)}END{print SUM}')
frag_bytes_24k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 20480 && $3 <= 24576) SUM+=(24576 - $3)}END{print SUM}')
frag_bytes_20k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 16384 && $3 <= 20480) SUM+=(20480 - $3)}END{print SUM}')
frag_bytes_16k=$(cat flush | grep "ALLOC" | awk '{if ($3 > 14336 && $3 <= 16384) SUM+=(16384 - $3)}END{print SUM}')

avg_frag_bytes_64k=$(($frag_bytes_64k/$num_64k))
avg_frag_bytes_56k=$(($frag_bytes_56k/$num_56k))
avg_frag_bytes_48k=$(($frag_bytes_48k/$num_48k))
avg_frag_bytes_40k=$(($frag_bytes_40k/$num_40k))
avg_frag_bytes_32k=$(($frag_bytes_32k/$num_32k))
avg_frag_bytes_28k=$(($frag_bytes_28k/$num_28k))
avg_frag_bytes_24k=$(($frag_bytes_24k/$num_24k))
avg_frag_bytes_20k=$(($frag_bytes_20k/$num_20k))
avg_frag_bytes_16k=$(($frag_bytes_16k/$num_16k))

num_bounded_allocs=$(($num_64k + $num_56k + $num_48k + $num_40k + $num_32k + $num_28k + $num_24k + $num_20k + $num_16k))
avg_frag_bytes=$((($avg_frag_bytes_64k*$num_64k + $avg_frag_bytes_56k*$num_56k + $avg_frag_bytes_48k*$num_48k + $avg_frag_bytes_40k*$num_40k + $avg_frag_bytes_32k*$num_32k + $avg_frag_bytes_28k*$num_28k + $avg_frag_bytes_24k*$num_24k + $avg_frag_bytes_20k*$num_20k + $avg_frag_bytes_16k*$num_16k)/$num_bounded_allocs))

# 48 was manually selected since it took ~8 minutes for the cache to be saturated
jemalloc_alloc=$(cat flush | grep "CGO A" | tail -n +48 | awk '{SUM+=$3}END{printf "%f\n", SUM/NR/(1024*1024*1024)}')
jemalloc_metadata=$(cat flush | grep "CGO M" | tail -n +48 | awk '{SUM+=$3}END{printf "%f\n", SUM/NR/(1024*1024)}')
jemalloc_resident=$(cat flush | grep "CGO T" | tail -n +48 | awk '{SUM+=$3}END{printf "%f\n", SUM/NR/(1024*1024*1024)}')

echo -e "Allocations: $num_allocs\nAverage Allocation Size (B): $avg_bytes\nBins\n\tSize(KiB)\tCount\tFragmentation(B/Alloc)\n\t64\t$num_64k\t$avg_frag_bytes_64k\n\t56\t$num_56k\t$avg_frag_bytes_56k\n\t48\t$num_48k\t$avg_frag_bytes_48k\n\t40\t$num_40k\t$avg_frag_bytes_40k\n\t32\t$num_32k\t$avg_frag_bytes_32k\n\t28\t$num_28k\t$avg_frag_bytes_28k\n\t24\t$num_24k\t$avg_frag_bytes_24k\n\t20\t$num_20k\t$avg_frag_bytes_20k\n\t16\t$num_16k\t$avg_frag_bytes_16k\n\navg(Fragmentation): $avg_frag_bytes\n"
echo -e "Jemalloc Stats\nAverage Allocated Bytes (GiB): $jemalloc_alloc\nAverage Resident Bytes (GiB): $jemalloc_resident\nAverage Metadata Bytes (MiB): $jemalloc_metadata"
