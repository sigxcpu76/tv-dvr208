import { MOTION_COLS, MOTION_ROWS, MotionGridMatrix } from './models';

export function createMotionGrid(fill = false): MotionGridMatrix {
  return Array.from({ length: MOTION_ROWS }, () =>
    Array.from({ length: MOTION_COLS }, () => fill)
  );
}

export function cloneMotionGrid(grid: MotionGridMatrix): MotionGridMatrix {
  return grid.map((row) => [...row]);
}

export function setMotionCell(
  grid: MotionGridMatrix,
  row: number,
  col: number,
  value: boolean
): MotionGridMatrix {
  const next = cloneMotionGrid(grid);
  if (row >= 0 && row < MOTION_ROWS && col >= 0 && col < MOTION_COLS) {
    next[row][col] = value;
  }
  return next;
}

export function countActiveCells(grid: MotionGridMatrix): number {
  let count = 0;
  for (let r = 0; r < grid.length; r++) {
    for (let c = 0; c < (grid[r]?.length ?? 0); c++) {
      if (grid[r][c]) count++;
    }
  }
  return count;
}

export function motionGridToHex(grid: MotionGridMatrix): string {
  let hex = '';
  for (let r = 0; r < MOTION_ROWS; r++) {
    let rowVal = 0;
    for (let c = 0; c < MOTION_COLS; c++) {
      if (grid[r]?.[c]) {
        rowVal |= 1 << (23 - c);
      }
    }
    hex += (rowVal >>> 0).toString(16).padStart(6, '0');
  }
  return hex;
}

export function parseMotionGridHex(hex: string): MotionGridMatrix {
  const cleaned = hex.trim();
  if (cleaned.length !== MOTION_ROWS * 6) {
    return createMotionGrid(false);
  }
  const grid = createMotionGrid(false);
  for (let r = 0; r < MOTION_ROWS; r++) {
    const rowHex = cleaned.slice(r * 6, (r + 1) * 6);
    const val = parseInt(rowHex, 16);
    if (Number.isNaN(val)) continue;
    for (let c = 0; c < MOTION_COLS; c++) {
      grid[r][c] = ((val >>> (23 - c)) & 1) === 1;
    }
  }
  return grid;
}
