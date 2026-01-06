import { defineType, defineField } from 'sanity'

export const chess = defineType({
  name: 'chess',
  title: 'Chess Game',
  type: 'object',
  fields: [
    defineField({
      name: 'pgn',
      title: 'PGN String',
      type: 'text',
      description: 'Paste the game PGN here (e.g., 1. e4 e5 2. Nf3...)',
      validation: (Rule) => Rule.required(),
    }),
    defineField({
      name: 'title',
      title: 'Game Title (Optional)',
      type: 'string',
      initialValue: 'White vs Black',
    }),
  ],
  preview: {
    select: {
      title: 'title',
      subtitle: 'pgn',
    },
    prepare(selection) {
      return {
        title: selection.title || 'Chess Game',
        subtitle: 'Interactive Board',
      }
    },
  },
})