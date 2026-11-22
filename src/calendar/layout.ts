import { DAY, type Block } from "../../shared/timeline.ts";

export type Segment = {
  block: Block;
  start: number;
  end: number;
  lane: number;
  lanes: number;
};

export function partition(blocks: Block[], day: number): Segment[] {
  const segments = blocks
    .filter((block) => block.start < day + DAY && block.end > day)
    .map((block) => ({
      block,
      start: Math.max(day, block.start),
      end: Math.min(day + DAY, block.end),
      lane: 0,
      lanes: 1,
    }))
    .sort((a, b) => a.start - b.start || b.end - a.end);
  let ends: number[] = [];
  let groupStart = 0;
  let groupEnd = day;
  function closeGroup(until: number) {
    for (const segment of segments.slice(groupStart, until)) segment.lanes = ends.length;
  }
  for (const [index, segment] of segments.entries()) {
    if (segment.start >= groupEnd) {
      closeGroup(index);
      groupStart = index;
      ends = [];
    }
    const available = ends.findIndex((end) => end <= segment.start);
    segment.lane = available === -1 ? ends.length : available;
    ends[segment.lane] = segment.end;
    groupEnd = Math.max(groupEnd, segment.end);
  }
  closeGroup(segments.length);
  return segments;
}
