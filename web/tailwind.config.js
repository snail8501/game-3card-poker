/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [ './src/**/*.jsx' ],
  theme: {
    extend: {
      keyframes: {
        floating: {
          '0%, 100%': { transform: 'translate3d(0px,0px,0px)' },
          '50%': { transform: 'translate3d(0px,20px,0px)' },
        },
      },
      animation: {
        floating: 'floating 2s ease-in-out infinite',
      },
    },
  },
  plugins: [ require('@tailwindcss/forms') ],
}

