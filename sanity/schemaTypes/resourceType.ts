import {DocumentIcon} from '@sanity/icons'
import {defineField, defineType} from 'sanity'

export const resourceType = defineType({
  name: 'resource',
  title: 'Resource',
  type: 'document',
  icon: DocumentIcon,
  fields: [
    defineField({
      name: 'title',
      type: 'string',
      validation: (Rule) => Rule.required(),
    }),
    defineField({
      name: 'slug',
      type: 'slug',
      options: {source: 'title'},
      validation: (Rule) => Rule.required(),
    }),
    defineField({
      name: 'summary',
      type: 'text',
      rows: 3,
      validation: (Rule) => Rule.required().max(320),
    }),
    defineField({
      name: 'resourceType',
      title: 'Resource type',
      type: 'string',
      options: {
        list: [
          {title: 'Article', value: 'article'},
          {title: 'Worksheet', value: 'worksheet'},
          {title: 'Guide', value: 'guide'},
        ],
      },
      validation: (Rule) => Rule.required(),
    }),
    defineField({
      name: 'publishedAt',
      type: 'datetime',
      validation: (Rule) => Rule.required().min('2000-01-01T00:00:00Z'),
    }),
    defineField({
      name: 'body',
      type: 'blockContent',
    }),
    defineField({
      name: 'download',
      title: 'Downloadable file',
      type: 'file',
    }),
    defineField({
      name: 'externalUrl',
      title: 'External URL',
      type: 'url',
      validation: (Rule) => Rule.uri({scheme: ['https']}),
    }),
  ],
})
