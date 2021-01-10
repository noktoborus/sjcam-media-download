# sjcam-media-download
Example SJCAM API for get photos and videos from SJCAM SJ8

Program can:
* download full-size videos and photos. Thumbs ignored.
* continue download if interrupted in previous run
* print camera info
* print battery status between downloads

# continue-download
I do not known why origin SJCAM app (SJCAM Zone) in get file message use field 'Offset_value':

{"Offset_value":0,"param":"\/tmp\/SD0\/DCIM\/100MEDIA\/20210108204256_0001.THM","msg_id":1285,"token":11}

correct name of this field is "offset" (as firmware from SJ8PRO and JS10PRO think). It work and it is amazing.