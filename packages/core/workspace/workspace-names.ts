export const WORKSPACE_NAME_SERIES = [
  "workshop", "computing", "ai", "space", "nature", "voyage",
] as const;

export type WorkspaceNameSeries = (typeof WORKSPACE_NAME_SERIES)[number];
export type WorkspaceNameSelection = WorkspaceNameSeries | "all";
export const DEFAULT_WORKSPACE_NAME_SERIES: WorkspaceNameSeries = "workshop";

export interface WorkspaceName {
  id: string;
  en: string;
  zh: string;
  slug: string;
}

function names(entries: readonly (readonly [string, string, string])[]): readonly WorkspaceName[] {
  return entries.map(([slug, en, zh]) => ({ id: slug, slug, en, zh }));
}

export const WORKSPACE_NAMES: Readonly<Record<WorkspaceNameSeries, readonly WorkspaceName[]>> = {
  workshop: names([
    ["code-workshop", "Code Workshop", "代码工坊"],
    ["pixel-works", "Pixel Works", "像素工场"],
    ["build-lab", "Build Lab", "构建实验室"],
    ["prototype-studio", "Prototype Studio", "原型空间"],
    ["patch-studio", "Patch Studio", "补丁工坊"],
    ["logic-works", "Logic Works", "逻辑工场"],
    ["script-shed", "Script Shed", "脚本小屋"],
    ["bit-foundry", "Bit Foundry", "比特铸坊"],
    ["release-dock", "Release Dock", "发布码头"],
    ["interface-lab", "Interface Lab", "接口实验室"],
    ["debug-den", "Debug Den", "调试小站"],
    ["branch-studio", "Branch Studio", "分支工坊"],
    ["merge-room", "Merge Room", "合并空间"],
    ["test-bench", "Test Bench", "测试台"],
    ["compile-club", "Compile Club", "编译社"],
    ["source-studio", "Source Studio", "源码工坊"],
    ["sprint-garage", "Sprint Garage", "迭代车间"],
    ["design-bench", "Design Bench", "设计工台"],
    ["byte-bakery", "Byte Bakery", "字节面包房"],
    ["function-forge", "Function Forge", "函数锻坊"],
  ]),
  computing: names([
    ["recursion", "Recursion", "递归"],
    ["closure", "Closure", "闭包"],
    ["kernel", "Kernel", "内核"],
    ["matrix", "Matrix", "矩阵"],
    ["topology", "Topology", "拓扑"],
    ["lambda", "Lambda", "拉姆达"],
    ["vector", "Vector", "向量"],
    ["binary", "Binary", "二进制"],
    ["stack", "Stack", "栈"],
    ["queue", "Queue", "队列"],
    ["hash", "Hash", "哈希"],
    ["graph", "Graph", "图谱"],
    ["tensor", "Tensor", "张量"],
    ["pipeline", "Pipeline", "流水线"],
    ["coroutine", "Coroutine", "协程"],
    ["cache", "Cache", "缓存"],
    ["index", "Index", "索引"],
    ["signal", "Signal", "信号"],
    ["predicate", "Predicate", "谓词"],
    ["invariant", "Invariant", "不变量"],
  ]),
  ai: names([
    ["insight", "Insight", "灵犀"],
    ["spark", "Spark", "火花"],
    ["echo", "Echo", "回声"],
    ["emergence", "Emergence", "涌现"],
    ["neural-garden", "Neural Garden", "神经花园"],
    ["latent-space", "Latent Space", "潜在空间"],
    ["synapse", "Synapse", "突触"],
    ["gradient", "Gradient", "梯度"],
    ["context", "Context", "上下文"],
    ["token-orchard", "Token Orchard", "词元果园"],
    ["prompt-lab", "Prompt Lab", "提示实验室"],
    ["reasoning-room", "Reasoning Room", "推理空间"],
    ["embedding", "Embedding", "嵌入"],
    ["attention", "Attention", "注意力"],
    ["perception", "Perception", "感知"],
    ["thought-trail", "Thought Trail", "思路小径"],
    ["learning-grove", "Learning Grove", "学习林"],
    ["seed-model", "Seed Model", "种子模型"],
    ["dream-engine", "Dream Engine", "梦想引擎"],
    ["idea-field", "Idea Field", "灵感原野"],
  ]),
  space: names([
    ["polaris", "Polaris", "北辰"],
    ["sirius", "Sirius", "天狼星"],
    ["vega", "Vega", "织女星"],
    ["andromeda", "Andromeda", "仙女座"],
    ["orion", "Orion", "猎户座"],
    ["altair", "Altair", "牛郎星"],
    ["lyra", "Lyra", "天琴座"],
    ["cygnus", "Cygnus", "天鹅座"],
    ["pegasus", "Pegasus", "飞马座"],
    ["cassiopeia", "Cassiopeia", "仙后座"],
    ["pleiades", "Pleiades", "昴星团"],
    ["milky-way", "Milky Way", "银河"],
    ["nebula", "Nebula", "星云"],
    ["comet", "Comet", "彗星"],
    ["luna", "Luna", "月球"],
    ["mars", "Mars", "火星"],
    ["jupiter", "Jupiter", "木星"],
    ["saturn", "Saturn", "土星"],
    ["neptune", "Neptune", "海王星"],
    ["titan", "Titan", "泰坦"],
  ]),
  nature: names([
    ["pine-breeze", "Pine Breeze", "松风"],
    ["mountain-mist", "Mountain Mist", "山岚"],
    ["green-isle", "Green Isle", "青屿"],
    ["creek-valley", "Creek Valley", "溪谷"],
    ["spruce", "Spruce", "云杉"],
    ["cedar", "Cedar", "雪松"],
    ["bamboo-grove", "Bamboo Grove", "竹林"],
    ["moss", "Moss", "青苔"],
    ["fern", "Fern", "蕨叶"],
    ["ginkgo", "Ginkgo", "银杏"],
    ["morning-dew", "Morning Dew", "晨露"],
    ["spring-rain", "Spring Rain", "春雨"],
    ["wildflower", "Wildflower", "野花"],
    ["meadow", "Meadow", "草甸"],
    ["snowpeak", "Snowpeak", "雪峰"],
    ["tidepool", "Tidepool", "潮池"],
    ["coral", "Coral", "珊瑚"],
    ["firefly", "Firefly", "萤火"],
    ["kingfisher", "Kingfisher", "翠鸟"],
    ["sequoia", "Sequoia", "红杉"],
  ]),
  voyage: names([
    ["far-sail", "Far Sail", "远帆"],
    ["lighthouse", "Lighthouse", "灯塔"],
    ["compass", "Compass", "罗盘"],
    ["horizon", "Horizon", "地平线"],
    ["set-sail", "Set Sail", "启航"],
    ["waypoint", "Waypoint", "航点"],
    ["pathfinder", "Pathfinder", "探路者"],
    ["trailhead", "Trailhead", "山径起点"],
    ["basecamp", "Basecamp", "大本营"],
    ["expedition", "Expedition", "远征"],
    ["harbor", "Harbor", "港湾"],
    ["trade-wind", "Trade Wind", "信风"],
    ["sextant", "Sextant", "六分仪"],
    ["star-chart", "Star Chart", "星图"],
    ["outpost", "Outpost", "前哨"],
    ["new-world", "New World", "新大陆"],
    ["tailwind", "Tailwind", "顺风"],
    ["longship", "Longship", "长船"],
    ["odyssey", "Odyssey", "漫游记"],
    ["open-water", "Open Water", "开阔海域"],
  ]),
};

export function isWorkspaceNameSelection(value: unknown): value is WorkspaceNameSelection {
  return value === "all" || WORKSPACE_NAME_SERIES.some((series) => series === value);
}

const SUFFIX_ALPHABET = "abcdefghijklmnopqrstuvwxyz0123456789";

/** One instance belongs to one creation session; UI preferences never persist this history. */
export function createWorkspaceNameGenerator(random: () => number = Math.random) {
  const seen = new Set<string>();
  let lastId: string | undefined;

  return (locale: string, selection: WorkspaceNameSelection): { id: string; name: string; slug: string } => {
    const series = selection === "all" ? WORKSPACE_NAME_SERIES : [selection];
    const unseen = (entry: WorkspaceName) => !seen.has(entry.id) && entry.id !== lastId;
    let available = series.map((key) => WORKSPACE_NAMES[key].filter(unseen)).filter((pool) => pool.length > 0);

    if (available.length === 0) {
      for (const key of series) {
        for (const entry of WORKSPACE_NAMES[key]) seen.delete(entry.id);
      }
      available = series.map((key) => WORKSPACE_NAMES[key].filter(unseen)).filter((pool) => pool.length > 0);
    }

    // Choose a series first so a larger catalog never dominates All.
    const pool = selection === "all" ? available[Math.floor(random() * available.length)]! : available[0]!;
    const entry = pool[Math.floor(random() * pool.length)]!;
    seen.add(entry.id);
    lastId = entry.id;

    let suffix = "";
    for (let index = 0; index < 4; index += 1) {
      suffix += SUFFIX_ALPHABET[Math.floor(random() * SUFFIX_ALPHABET.length)];
    }

    return { id: entry.id, name: locale.startsWith("zh") ? entry.zh : entry.en, slug: `${entry.slug}-${suffix}` };
  };
}
