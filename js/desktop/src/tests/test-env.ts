export type TestEnv = {
  addr: string;
  internalAddr: string;
  privateKey: string;
  publicAddress: string;
  multibaseKey: string;
  testDuration: number;
  env: Record<string, string>;
};

export type E2ETest = {
  setup?: (testEnv: TestEnv) => Promise<TestEnv>;
  test: (testEnv: TestEnv) => Promise<string | null>;
};
