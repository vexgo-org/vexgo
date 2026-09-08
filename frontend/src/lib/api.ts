export const unwrap = <T>(p: Promise<{ data: T }>) => p.then((r) => r.data);
