import { createRequire } from "module";
const require = createRequire(import.meta.url);

const config = {
  plugins: {
    "postcss-import": {
      // Resolve package @imports from node_modules
      resolve: (id, basedir) => {
        try {
          return require.resolve(id, { paths: [basedir || process.cwd()] });
        } catch {
          return id;
        }
      },
    },
    tailwindcss: {},
    autoprefixer: {},
  },
};

export default config;
