# Go-image-resizer
Tool to resize an images through the command line.

Dependencies: [github.com/nfnt/resize](github.com/nfnt/resize)

Usage of resizer:

    -d string
    Sets the path to dir containing images.
      
    -p string
    Sets the path to save resized images. Default value -d flag value.
  
    -r	
    Sets for recursively obtain all images in directory.
    
    -sc int
    Set x times scaling value. Negative means downscaling. Positive upscaling. (default 1)

Example of usage:

    $ resizer -r -d /home/John/Pictures -p /home/John/ResizedPictures -sc 2

Recursively resize all images by 50% in **/home/John/Pictures** and save it in **/home/John/ResizedPictures** 
